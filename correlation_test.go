package sixthsenseGoAgent_test

import (
	"context"
	"log"
	"reflect"
	"testing"

	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent"
	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent/propagation"
	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent/reporter"
)

const (
	correlationTestKey   = "test-key"
	correlationTestValue = "test-value"
)

func TestGetCorrelation_WithTracingContest(t *testing.T) {
	verifyPutResult := func(ctx context.Context, key, value string, result bool, t *testing.T) {
		if success := sixthsenseGoAgent.PutCorrelation(ctx, key, value); success != result {
			t.Errorf("put correlation result is not right: %t", success)
		}
	}
	tests := []struct {
		name string
		// extract from context
		extractor propagation.Extractor
		// extract correlation context
		extracted map[string]string
		// put correlation
		customCase func(ctx context.Context, t *testing.T)
		// after exported correaltion context
		want map[string]string
	}{
		{
			name: "no context",
			extractor: func(headerKey string) (string, error) {
				return "", nil
			},
			extracted: make(map[string]string),
			customCase: func(ctx context.Context, t *testing.T) {
				verifyPutResult(ctx, correlationTestKey, correlationTestValue, true, t)
			},
			want: func() map[string]string {
				m := make(map[string]string)
				m[correlationTestKey] = correlationTestValue
				return m
			}(),
		},
		{
			name: "existing context with correlation",
			extractor: func(headerKey string) (string, error) {
				if headerKey == propagation.HeaderCorrelation {
					// test1 = t1
					return "dGVzdDE=:dDE=", nil
				}
				if headerKey == propagation.Header {
					return "1-MWYyZDRiZjQ3YmY3MTFlYWI3OTRhY2RlNDgwMDExMjI=-MWU3YzIwNGE3YmY3MTFlYWI4NThhY2RlNDgwMDExMjI=" +
						"-0-c2VydmljZQ==-aW5zdGFuY2U=-cHJvcGFnYXRpb24=-cHJvcGFnYXRpb246NTU2Ng==", nil
				}
				return "", nil
			},
			extracted: func() map[string]string {
				m := make(map[string]string)
				m["test1"] = "t1"
				return m
			}(),
			customCase: func(ctx context.Context, t *testing.T) {
				verifyPutResult(ctx, correlationTestKey, correlationTestValue, true, t)
			},
			want: func() map[string]string {
				m := make(map[string]string)
				m[correlationTestKey] = correlationTestValue
				m["test1"] = "t1"
				return m
			}(),
		},
		{
			name: "existing context with empty correlation",
			extractor: func(headerKey string) (string, error) {
				if headerKey == propagation.HeaderCorrelation {
					return "", nil
				}
				if headerKey == propagation.Header {
					return "1-MWYyZDRiZjQ3YmY3MTFlYWI3OTRhY2RlNDgwMDExMjI=-MWU3YzIwNGE3YmY3MTFlYWI4NThhY2RlNDgwMDExMjI=" +
						"-0-c2VydmljZQ==-aW5zdGFuY2U=-cHJvcGFnYXRpb24=-cHJvcGFnYXRpb246NTU2Ng==", nil
				}
				return "", nil
			},
			extracted: func() map[string]string {
				m := make(map[string]string)
				return m
			}(),
			customCase: func(ctx context.Context, t *testing.T) {
				verifyPutResult(ctx, correlationTestKey, correlationTestValue, true, t)
			},
			want: func() map[string]string {
				m := make(map[string]string)
				m[correlationTestKey] = correlationTestValue
				return m
			}(),
		},
		{
			name: "empty context with put bound judge",
			extractor: func(headerKey string) (string, error) {
				return "", nil
			},
			customCase: func(ctx context.Context, t *testing.T) {
				// empty key
				verifyPutResult(ctx, "", "123", false, t)

				// remove key
				verifyPutResult(ctx, correlationTestKey, correlationTestValue, true, t)
				verifyPutResult(ctx, correlationTestKey, "", true, t)
				if sixthsenseGoAgent.GetCorrelation(ctx, correlationTestKey) != "" {
					t.Errorf("correlation test key should be null")
				}

				// out of max value size
				verifyPutResult(ctx, "test-key", "1234567890123456", false, t)

				// out of key count
				verifyPutResult(ctx, "test-key1", "123", true, t)
				verifyPutResult(ctx, "test-key2", "123", true, t)
				verifyPutResult(ctx, "test-key3", "123", true, t)
				verifyPutResult(ctx, "test-key4", "123", false, t)

				// exists key
				verifyPutResult(ctx, "test-key1", "123456", true, t)
			},
			want: func() map[string]string {
				m := make(map[string]string)
				m["test-key1"] = "123456"
				m["test-key2"] = "123"
				m["test-key3"] = "123"
				return m
			}(),
		},
	}

	r, err := reporter.NewLogReporter()
	if err != nil {
		log.Fatalf("new reporter error %v \n", err)
	}
	defer r.Close()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			tracer, _ := sixthsenseGoAgent.NewTracer("correlationTest", sixthsenseGoAgent.WithReporter(r), sixthsenseGoAgent.WithSampler(1), sixthsenseGoAgent.WithCorrelation(3, 10))

			// create entry span from extractor
			span, ctx, _ := tracer.CreateEntrySpan(ctx, "test-entry", tt.extractor)
			defer span.End()

			// verify extracted context is same
			if tt.extracted != nil {
				for key, value := range tt.extracted {
					if sixthsenseGoAgent.GetCorrelation(ctx, key) != value {
						t.Errorf("error get previous correlation value, current is: %s", sixthsenseGoAgent.GetCorrelation(ctx, key))
					}
				}
			}

			// custom case
			tt.customCase(ctx, t)

			// put sample local span
			span, ctx, _ = tracer.CreateLocalSpan(ctx)
			defer span.End()

			// validate correlation context
			// verify extracted context is same
			for key, value := range tt.want {
				if sixthsenseGoAgent.GetCorrelation(ctx, key) != value {
					t.Errorf("error validate correlation value, current is: %s", sixthsenseGoAgent.GetCorrelation(ctx, key))
				}
			}

			// export context
			scx := propagation.SpanContext{}
			_, err = tracer.CreateExitSpan(ctx, "test-exit", "127.0.0.1:8080", func(headerKey, headerValue string) error {
				if headerKey == propagation.HeaderCorrelation {
					err = scx.DecodeSW8Correlation(headerValue)
					if err != nil {
						t.Fail()
					}
				}
				return nil
			})
			if err != nil {
				t.Fail()
			}
			reflect.DeepEqual(scx, tt.want)
		})
	}
}

func TestGetCorrelation_WithEmptyContext(t *testing.T) {
	emptyValue := sixthsenseGoAgent.GetCorrelation(context.Background(), "empty-key")
	if emptyValue != "" {
		t.Errorf("should be empty value")
	}

	success := sixthsenseGoAgent.PutCorrelation(context.Background(), "empty-key", "empty-value")
	if success {
		t.Errorf("put correlation key should be failed")
	}

	emptyValue = sixthsenseGoAgent.GetCorrelation(context.Background(), "empty-key")
	if emptyValue != "" {
		t.Errorf("should be empty value")
	}
}

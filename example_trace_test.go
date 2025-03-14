package sixthsenseGoAgent_test

import (
	"context"
	"log"
	"time"

	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent"
	"github.com/apache/skywalking-go/reporter"
)

func ExampleNewTracer() {
	// Use gRPC reporter for production
	r, err := reporter.NewLogReporter()
	if err != nil {
		log.Fatalf("new reporter error %v \n", err)
	}
	defer r.Close()
	tracer, err := sixthsenseGoAgent.NewTracer("example", sixthsenseGoAgent.WithReporter(r))
	if err != nil {
		log.Fatalf("create tracer error %v \n", err)
	}
	// This for test
	span, ctx, err := tracer.CreateLocalSpan(context.Background())
	if err != nil {
		log.Fatalf("create new local span error %v \n", err)
	}
	span.SetOperationName("invoke data")
	span.Tag("kind", "outer")
	time.Sleep(time.Second)
	subSpan, _, err := tracer.CreateLocalSpan(ctx)
	if err != nil {
		log.Fatalf("create new sub local span error %v \n", err)
	}
	subSpan.SetOperationName("invoke inner")
	subSpan.Log(time.Now(), "inner", "this is right")
	time.Sleep(time.Second)
	subSpan.End()
	time.Sleep(500 * time.Millisecond)
	span.End()
	time.Sleep(time.Second)
	// Output:
}

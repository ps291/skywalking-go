package http

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent"
	//"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent/reporter"
)

func ExampleNewServerMiddleware() {
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

	sm, err := NewServerMiddleware(tracer)
	if err != nil {
		log.Fatalf("create server middleware error %v \n", err)
	}

	client, err := NewClient(tracer)
	if err != nil {
		log.Fatalf("create client error %v \n", err)
	}

	// create test server
	ts := httptest.NewServer(sm(endFunc()))
	defer ts.Close()

	// call end service
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/end", ts.URL), nil)
	if err != nil {
		log.Fatalf("unable to create http request: %+v\n", err)
	}
	res, err := client.Do(request)
	if err != nil {
		log.Fatalf("unable to do http request: %+v\n", err)
	}
	_ = res.Body.Close()
	time.Sleep(time.Second)

	// Output:
}

func endFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("end func called with method: %s\n", r.Method)
		time.Sleep(50 * time.Millisecond)
	}
}

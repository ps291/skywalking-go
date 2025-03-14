package reporter

import (
	"encoding/json"
	"log"
	"os"

	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent"
)

func NewLogReporter() (sixthsenseGoAgent.Reporter, error) {
	return &logReporter{logger: log.New(os.Stderr, "sixthsenseGoAgent-log", log.LstdFlags)}, nil
}

type logReporter struct {
	logger *log.Logger
}

func (lr *logReporter) Boot(service string, serviceInstance string, cdsWatchers []sixthsenseGoAgent.AgentConfigChangeWatcher) {

}

func (lr *logReporter) Send(spans []sixthsenseGoAgent.ReportedSpan) {
	if spans == nil {
		return
	}
	b, err := json.Marshal(spans)
	if err != nil {
		lr.logger.Printf("Error: %s", err)
		return
	}
	root := spans[len(spans)-1]
	lr.logger.Printf("Segment-%v: %s \n", root.Context().SegmentID, b)
}

func (lr *logReporter) Close() {
	lr.logger.Println("Close log reporter")
}

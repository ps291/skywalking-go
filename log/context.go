package log

import (
	"context"
	"fmt"

	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent"
)

type SkyWalkingContext struct {
	ServiceName         string
	ServiceInstanceName string
	TraceID             string
	TraceSegmentID      string
	SpanID              int32
}

// FromContext from context for logging
func FromContext(ctx context.Context) *SkyWalkingContext {
	return &SkyWalkingContext{
		ServiceName:         sixthsenseGoAgent.ServiceName(ctx),
		ServiceInstanceName: sixthsenseGoAgent.ServiceInstanceName(ctx),
		TraceID:             sixthsenseGoAgent.TraceID(ctx),
		TraceSegmentID:      sixthsenseGoAgent.TraceSegmentID(ctx),
		SpanID:              sixthsenseGoAgent.SpanID(ctx),
	}
}

func (context *SkyWalkingContext) String() string {
	return fmt.Sprintf("[%s,%s,%s,%s,%d]", context.ServiceName, context.ServiceInstanceName,
		context.TraceID, context.TraceSegmentID, context.SpanID)
}

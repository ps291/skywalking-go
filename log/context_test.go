package log

import (
	"context"
	"fmt"
	"testing"
)

func TestFromContext(t *testing.T) {
	ctx := context.Background()
	skyWalkingContext := FromContext(ctx)
	verifyContext(t, skyWalkingContext)
}

func verifyContext(t *testing.T, ctx *SkyWalkingContext) {
	if ctx == nil {
		t.Error("nil context")
		return
	}

	contextString := ctx.String()
	if contextString == "" {
		t.Error("empty context string")
	}

	exceptString := fmt.Sprintf("[%s,%s,%s,%s,%d]", ctx.ServiceName, ctx.ServiceInstanceName,
		ctx.TraceID, ctx.TraceSegmentID, ctx.SpanID)
	if contextString != exceptString {
		t.Errorf("wrong context string, excepted:%s, actual:%s", exceptString, contextString)
	}
}

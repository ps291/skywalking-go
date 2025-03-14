package sixthsenseGoAgent

import "context"

type CorrelationConfig struct {
	MaxKeyCount  int
	MaxValueSize int
}

func WithCorrelation(keyCount, valueSize int) TracerOption {
	return func(t *Tracer) {
		t.correlation = &CorrelationConfig{
			MaxKeyCount:  keyCount,
			MaxValueSize: valueSize,
		}
	}
}

func PutCorrelation(ctx context.Context, key, value string) bool {
	if key == "" {
		return false
	}

	activeSpan := ctx.Value(ctxKeyInstance)
	if activeSpan == nil {
		return false
	}

	span, ok := activeSpan.(segmentSpan)
	if !ok {
		return false
	}
	correlationContext := span.context().CorrelationContext
	// remove key
	if value == "" {
		delete(correlationContext, key)
		return true
	}
	// out of max value size
	if len(value) > span.tracer().correlation.MaxValueSize {
		return false
	}
	// already exists key
	if _, ok := correlationContext[key]; ok {
		correlationContext[key] = value
		return true
	}
	// out of max key count
	if len(correlationContext) >= span.tracer().correlation.MaxKeyCount {
		return false
	}
	span.context().CorrelationContext[key] = value
	return true
}

func GetCorrelation(ctx context.Context, key string) string {
	activeSpan := ctx.Value(ctxKeyInstance)
	if activeSpan == nil {
		return ""
	}

	span, ok := activeSpan.(segmentSpan)
	if !ok {
		return ""
	}
	return span.context().CorrelationContext[key]
}

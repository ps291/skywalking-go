package http

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent"
	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent/internal/tool"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	errInvalidTracer = tool.Error("invalid tracer")
)

const componentIDGOHttpServer = 5004

type handler struct {
	tracer    *sixthsenseGoAgent.Tracer
	name      string
	next      http.Handler
	extraTags map[string]string
}

// ServerOption allows Middleware to be optionally configured.
type ServerOption func(*handler)

// Tag adds extra tag to server spans.
func WithServerTag(key string, value string) ServerOption {
	return func(h *handler) {
		if h.extraTags == nil {
			h.extraTags = make(map[string]string)
		}
		h.extraTags[key] = value
	}
}

// WithOperationName override default operation name.
func WithServerOperationName(name string) ServerOption {
	return func(h *handler) {
		h.name = name
	}
}

// NewServerMiddleware returns a http.Handler middleware with tracing.
func NewServerMiddleware(tracer *sixthsenseGoAgent.Tracer, options ...ServerOption) (func(http.Handler) http.Handler, error) {
	if tracer == nil {
		return nil, errInvalidTracer
	}
	return func(next http.Handler) http.Handler {
		h := &handler{
			tracer: tracer,
			next:   next,
		}
		for _, option := range options {
			option(h)
		}
		return h
	}, nil
}

// ServeHTTP implements http.Handler.
func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	span, ctx, err := h.tracer.CreateEntrySpan(r.Context(), getOperationName(h.name, r), func(key string) (string, error) {
		return r.Header.Get(key), nil
	})
	if err != nil {
		if h.next != nil {
			h.next.ServeHTTP(w, r)
		}
		return
	}
	span.SetComponent(componentIDGOHttpServer)
	for k, v := range h.extraTags {
		span.Tag(sixthsenseGoAgent.Tag(k), v)
	}
	span.Tag(sixthsenseGoAgent.TagHTTPMethod, r.Method)
	span.Tag(sixthsenseGoAgent.TagURL, fmt.Sprintf("%s%s", r.Host, r.URL.Path))
	span.SetSpanLayer(agentv3.SpanLayer_Http)

	rww := &responseWriterWrapper{w: w, statusCode: 200}
	defer func() {
		code := rww.statusCode
		if code >= 400 {
			span.Error(time.Now(), "Error on handling request")
		}
		span.Tag(sixthsenseGoAgent.TagStatusCode, strconv.Itoa(code))
		span.End()
	}()
	if h.next != nil {
		h.next.ServeHTTP(rww, r.WithContext(ctx))
	}
}

type responseWriterWrapper struct {
	w          http.ResponseWriter
	statusCode int
}

func (rww *responseWriterWrapper) Header() http.Header {
	return rww.w.Header()
}

func (rww *responseWriterWrapper) Write(bytes []byte) (int, error) {
	return rww.w.Write(bytes)
}

func (rww *responseWriterWrapper) WriteHeader(statusCode int) {
	rww.statusCode = statusCode
	rww.w.WriteHeader(statusCode)
}

func getOperationName(name string, r *http.Request) string {
	if name == "" {
		return fmt.Sprintf("/%s%s", r.Method, r.URL.Path)
	}
	return name
}

package http

import (
	"net/http"
	"strconv"
	"time"

	"artifacts-observability.sixthsense.rakuten.com/sixthsense/sixthsenseGoAgent"
	agentv3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const componentIDGOHttpClient = 5005

type ClientConfig struct {
	name      string
	client    *http.Client
	tracer    *sixthsenseGoAgent.Tracer
	extraTags map[string]string
}

// ClientOption allows optional configuration of Client.
type ClientOption func(*ClientConfig)

// WithOperationName override default operation name.
func WithClientOperationName(name string) ClientOption {
	return func(c *ClientConfig) {
		c.name = name
	}
}

// WithClientTag adds extra tag to client spans.
func WithClientTag(key string, value string) ClientOption {
	return func(c *ClientConfig) {
		if c.extraTags == nil {
			c.extraTags = make(map[string]string)
		}
		c.extraTags[key] = value
	}
}

// WithClient set customer http client.
func WithClient(client *http.Client) ClientOption {
	return func(c *ClientConfig) {
		c.client = client
	}
}

// NewClient returns an HTTP Client with tracer
func NewClient(tracer *sixthsenseGoAgent.Tracer, options ...ClientOption) (*http.Client, error) {
	if tracer == nil {
		return nil, errInvalidTracer
	}
	co := &ClientConfig{tracer: tracer}
	for _, option := range options {
		option(co)
	}
	if co.client == nil {
		co.client = &http.Client{}
	}
	tp := &transport{
		ClientConfig: co,
		delegated:    http.DefaultTransport,
	}
	if co.client.Transport != nil {
		tp.delegated = co.client.Transport
	}
	co.client.Transport = tp
	return co.client, nil
}

type transport struct {
	*ClientConfig
	delegated http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	span, err := t.tracer.CreateExitSpan(req.Context(), getOperationName(t.name, req), req.Host, func(key, value string) error {
		req.Header.Set(key, value)
		return nil
	})
	if err != nil {
		return t.delegated.RoundTrip(req)
	}
	defer span.End()
	span.SetComponent(componentIDGOHttpClient)
	for k, v := range t.extraTags {
		span.Tag(sixthsenseGoAgent.Tag(k), v)
	}
	span.Tag(sixthsenseGoAgent.TagHTTPMethod, req.Method)
	span.Tag(sixthsenseGoAgent.TagURL, req.URL.String())
	span.SetSpanLayer(agentv3.SpanLayer_Http)
	res, err = t.delegated.RoundTrip(req)
	if err != nil {
		span.Error(time.Now(), err.Error())
		return
	}
	span.Tag(sixthsenseGoAgent.TagStatusCode, strconv.Itoa(res.StatusCode))
	if res.StatusCode >= http.StatusBadRequest {
		span.Error(time.Now(), "Errors on handling client")
	}
	return res, nil
}

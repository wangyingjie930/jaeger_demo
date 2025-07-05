// internal/pkg/httpclient/client.go

package httpclient

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 是一个可追踪的、可注入的HTTP客户端
type Client struct {
	Tracer trace.Tracer
}

// NewClient 创建一个新的客户端实例
func NewClient(tracer trace.Tracer) *Client {
	return &Client{Tracer: tracer}
}

// Post 是 callService 的重构版本，作为 Client 的一个方法
func (c *Client) Post(ctx context.Context, serviceURL string, params url.Values) error {
	parsedURL, err := url.Parse(serviceURL)
	if err != nil {
		return err
	}
	// 从 URL 中解析出服务名用于 Span
	spanName := fmt.Sprintf("call-%s", strings.Split(parsedURL.Host, ":")[0])

	ctx, span := c.Tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	downstreamURL := *parsedURL
	q := downstreamURL.Query()
	for key, values := range params {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	downstreamURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", downstreamURL.String(), nil)
	if err != nil {
		span.RecordError(err)
		return err
	}

	span.SetAttributes(
		attribute.String("http.url", downstreamURL.String()),
		attribute.String("http.method", "POST"),
	)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("service %s returned status %s", serviceURL, resp.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	return nil
}

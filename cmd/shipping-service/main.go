package main

import (
	"context"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	// 使用 zerolog 替代标准 log
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

const (
	serviceName    = "shipping-service"
	jaegerEndpoint = "http://localhost:14268/api/traces"
)

var tracer = otel.Tracer(serviceName)

func main() {
	// 配置 zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zlog.Logger = zlog.With().Str("service", serviceName).Logger()

	tp, _ := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	defer tp.Shutdown(context.Background())

	// 使用一个中间件来注入 logger
	http.Handle("/get_quote", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 先提取trace上下文
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// 然后从提取的上下文中获取trace_id
		traceID := tracing.GetTraceIDFromContext(ctx)

		logger := zlog.With().Str("trace_id", traceID).Logger()
		// 将新的 logger 存入 context，以便 handler 使用
		ctx = logger.WithContext(ctx)

		// 调用真正的 handler
		handleGetQuote(w, r.WithContext(ctx))
	}))

	zlog.Info().Msg("Shipping Service listening on :8086")
	log.Fatal(http.ListenAndServe(":8086", nil))
}

func handleGetQuote(w http.ResponseWriter, r *http.Request) {
	// 从 context 中获取我们注入的 logger
	logger := zlog.Ctx(r.Context())

	// 这里不需要再次提取trace上下文，因为已经在中间件中处理了
	ctx := r.Context()
	_, span := tracer.Start(ctx, "shipping-service.GetQuote")
	defer span.End()

	logger.Info().Msg("Calculating shipping quote...") // 使用 zerolog

	time.Sleep(200 * time.Millisecond)
	span.AddEvent("Shipping quote calculated")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"cost": 10.0}`))
}

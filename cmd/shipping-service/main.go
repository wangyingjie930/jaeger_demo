package main

import (
	"github.com/wangyingjie930/nexus-pkg/bootstrap"
	"github.com/wangyingjie930/nexus-pkg/tracing"
	"go.opentelemetry.io/otel/propagation"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	// 使用 zerolog 替代标准 log
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

const (
	serviceName = "shipping-service"
)

var (
	tracer = otel.Tracer(serviceName)
)

func main() {
	bootstrap.Init()

	// 配置 zerolog
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zlog.Logger = zlog.With().Str("service", serviceName).Logger()

	bootstrap.StartService(bootstrap.AppInfo{
		ServiceName: serviceName,
		Port:        8086,
		RegisterHandlers: func(ctx bootstrap.AppCtx) {
			// 使用一个中间件来注入 logger
			ctx.Mux.Handle("/get_quote", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		},
	})
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

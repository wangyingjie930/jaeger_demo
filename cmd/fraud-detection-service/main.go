package main

import (
	"log"
	"net/http"
	"nexus/internal/pkg/bootstrap"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName = "fraud-detection-service"
)

func main() {
	bootstrap.Init()
	bootstrap.StartService(bootstrap.AppInfo{
		ServiceName: serviceName,
		Port:        8085,
		RegisterHandlers: func(ctx bootstrap.AppCtx) {
			ctx.Mux.HandleFunc("/check", handleFraudCheck)
		},
	})
}

func handleFraudCheck(w http.ResponseWriter, r *http.Request) {
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	var tracer = otel.Tracer(serviceName)
	_, span := tracer.Start(ctx, "fraud-detection-service.Check")
	defer span.End()
	log.Println("Performing fraud check...")
	time.Sleep(80 * time.Millisecond)
	span.AddEvent("Fraud check passed")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Fraud check passed"))
}

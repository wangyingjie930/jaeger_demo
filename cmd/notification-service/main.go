// cmd/notification-service/main.go
package main

import (
	"context"
	"jaeger-demo/internal/tracing"
	"log"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

const (
	serviceName    = "notification-service"
	jaegerEndpoint = "http://localhost:14268/api/traces"
)

var tracer = otel.Tracer(serviceName)

func main() {
	tp, err := tracing.InitTracerProvider(serviceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	http.HandleFunc("/send_notification", sendNotificationHandler)
	log.Println("Notification Service listening on :8083")
	log.Fatal(http.ListenAndServe(":8083", nil))
}

func sendNotificationHandler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	_, span := tracer.Start(ctx, "notification-service.Send")
	defer span.End()

	userID := r.URL.Query().Get("userID")
	span.SetAttributes(attribute.String("user.id", userID))

	// 模拟发送通知的耗时
	log.Printf("Sending notification to user %s", userID)
	time.Sleep(50 * time.Millisecond)
	span.AddEvent("Notification sent successfully")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Notification sent"))
}

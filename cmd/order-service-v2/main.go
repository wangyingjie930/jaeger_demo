// cmd/order-service/main.go
package main

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"jaeger-demo/internal/pkg/bootstrap"
	"net/http"
)

const (
	serviceName = "order-service-v2"
)

// main 函数是应用的"组装根" (Composition Root)
// 它的核心职责是：创建并组装所有依赖项，然后启动应用。
func main() {
	bootstrap.Init()

	bootstrap.StartService(bootstrap.AppInfo{
		ServiceName: serviceName,
		Port:        8081,
		RegisterHandlers: func(appCtx bootstrap.AppCtx) {
			appCtx.Mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
			appCtx.Mux.Handle("/metrics", promhttp.Handler())
			appCtx.Mux.HandleFunc("/create_complex_order", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted) // 202 Accepted 是一个非常适合此场景的状态码
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{
					"status":  "pending",
					"message": "this is v2",
				})
			})
		},
	})
}

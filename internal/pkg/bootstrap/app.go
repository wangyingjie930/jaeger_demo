// internal/pkg/bootstrap/app.go
package bootstrap

import (
	"context"
	"jaeger-demo/internal/pkg/nacos"
	"jaeger-demo/internal/pkg/tracing"
	"jaeger-demo/internal/pkg/utils"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type AppCtx struct {
	Mux   *http.ServeMux
	Nacos *nacos.Client
}

// AppInfo 包含了启动一个微服务所需的所有特定信息。
type AppInfo struct {
	ServiceName      string
	Port             int
	RegisterHandlers func(appCtx AppCtx) // 一个函数，允许每个服务注册自己独特的 HTTP 路由
}

// StartService 封装了所有微服务的通用启动和优雅关停逻辑。
func StartService(info AppInfo) {
	// 1. 从环境变量读取通用配置
	jaegerEndpoint := getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	nacosServerAddrs := getEnv("NACOS_SERVER_ADDRS", "localhost:8848")

	// 2. 初始化核心组件 (Tracer 和 Nacos Client)
	tp, err := tracing.InitTracerProvider(info.ServiceName, jaegerEndpoint)
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}

	nacosClient, err := nacos.NewNacosClient(nacosServerAddrs)
	if err != nil {
		log.Fatalf("failed to initialize nacos client: %v", err)
	}

	// 3. 获取本机 IP 用于注册
	ip, err := utils.GetOutboundIP()
	if err != nil {
		log.Fatalf("failed to get outbound IP address: %v", err)
	}

	// 4. 执行服务注册
	err = nacosClient.RegisterServiceInstance(info.ServiceName, ip, info.Port)
	if err != nil {
		log.Fatalf("failed to register service with nacos: %v", err)
	}

	// 5. 创建 HTTP Server 和 Mux
	mux := http.NewServeMux()
	if info.RegisterHandlers != nil {
		info.RegisterHandlers(AppCtx{Mux: mux, Nacos: nacosClient})
	}
	server := &http.Server{
		Addr:    ":" + strconv.Itoa(info.Port),
		Handler: mux,
	}

	// 6. 启动一个 goroutine 来监听 HTTP 服务
	go func() {
		log.Printf("%s listening on :%d", info.ServiceName, info.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not listen on %s: %v\n", server.Addr, err)
		}
	}()

	// 7. 设置优雅关停 (Graceful Shutdown)
	// 这是大厂实践中非常重要的一环，确保服务在退出前能完成清理工作
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞主 goroutine，直到接收到退出信号
	<-quit
	log.Printf("Shutting down service %s...", info.ServiceName)

	// 创建一个有超时的 context，用于关停流程
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 8. 在关停流程中，按顺序执行清理操作 (后进先出)
	// a. 从 Nacos 注销服务
	if err := nacosClient.DeregisterServiceInstance(info.ServiceName, ip, info.Port); err != nil {
		log.Printf("Error deregistering from Nacos: %v", err)
	} else {
		log.Printf("Service %s deregistered from Nacos.", info.ServiceName)
	}

	// b. 关闭 Tracer Provider，确保所有缓冲的 trace 都被发送出去
	if err := tp.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down tracer provider: %v", err)
	} else {
		log.Println("Tracer provider shut down.")
	}

	// c. 关闭 HTTP 服务器
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down http server: %v", err)
	} else {
		log.Println("HTTP server shut down.")
	}

	log.Printf("Service %s gracefully shut down.", info.ServiceName)
}

// getEnv 是一个内部辅助函数，从环境变量中读取配置。
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

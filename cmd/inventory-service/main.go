// cmd/inventory-service/main.go
package main

import (
	"context"
	"fmt"
	"jaeger-demo/internal/pkg/tracing"
	"jaeger-demo/internal/pkg/zookeeper"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

// getEnv 从环境变量中读取配置。
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

const (
	serviceName = "inventory-service"
)

var (
	jaegerEndpoint = getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces")
	// <<<< 2. 新增ZooKeeper连接地址的环境变量
	zkServersEnv = getEnv("ZK_SERVERS", "localhost:2181")
	tracer       = otel.Tracer(serviceName)
	zkConn       *zookeeper.Conn // <<<< 3. 定义一个全局的ZooKeeper连接变量
)

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

	// <<<< 4. 在服务启动时初始化ZooKeeper连接
	servers := strings.Split(zkServersEnv, ",")
	zkConn, err = zookeeper.InitZookeeper(servers)
	if err != nil {
		log.Fatalf("failed to initialize zookeeper connection: %v", err)
	}
	// 服务关闭时，也需要关闭ZK连接
	defer zkConn.Close()

	http.HandleFunc("/check_stock", checkStockHandler)
	http.HandleFunc("/reserve_stock", reserveStockHandler) // 新增：预占库存
	http.HandleFunc("/release_stock", releaseStockHandler) // 新增：释放库存
	// <<<<<<< 改造点结束 >>>>>>>>>

	log.Println("Inventory Service listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}

// reserveStockHandler 模拟预占库存 - 已改造
func reserveStockHandler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := tracer.Start(ctx, "inventory-service.ReserveStock")
	defer span.End()

	itemId := r.URL.Query().Get("itemId")
	quantityStr := r.URL.Query().Get("quantity")
	quantity, _ := strconv.Atoi(quantityStr)
	orderID := r.URL.Query().Get("orderId")

	span.SetAttributes(
		attribute.String("item.id", itemId),
		attribute.Int("item.quantity", quantity),
		attribute.String("order.id", orderID),
	)

	// <<<< 5. 在核心业务逻辑外层，加上分布式锁
	// 使用 itemId 作为锁的资源标识
	lock := zookeeper.NewDistributedLock(zkConn, itemId)
	log.Printf("Attempting to acquire lock for item %s, order %s", itemId, orderID)
	span.AddEvent("Acquiring distributed lock")

	if err := lock.Lock(); err != nil {
		log.Printf("ERROR: Failed to acquire lock for item %s: %v", itemId, err)
		http.Error(w, "Failed to acquire lock, please try again later", http.StatusServiceUnavailable)
		return
	}
	log.Printf("Successfully acquired lock for item %s, order %s", itemId, orderID)
	span.AddEvent("Acquired distributed lock")

	// 确保锁一定会被释放
	defer func() {
		log.Printf("Releasing lock for item %s, order %s", itemId, orderID)
		if err := lock.Unlock(); err != nil {
			// 在真实生产环境中，释放锁失败需要记录严重错误日志，并可能需要人工介入
			log.Printf("CRITICAL: Failed to release lock for item %s: %v", itemId, err)
		} else {
			span.AddEvent("Released distributed lock")
		}
	}()

	// ------------------ START: 原有的核心业务逻辑 ------------------
	// 故障注入点
	if itemId == "item-faulty-123" && quantity > 10 {
		log.Printf("Injecting fault for item %s", itemId)
		time.Sleep(500 * time.Millisecond)

		err := fmt.Errorf("inventory reserve failed for faulty item %s", itemId)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		http.Error(w, "Inventory service unavailable for this item", http.StatusInternalServerError)
		return
	}

	// 模拟数据库库存检查和扣减，这里现在是线程安全的了
	log.Println("Simulating stock check and deduction in database...")
	time.Sleep(100 * time.Millisecond)

	log.Printf("Stock reservation successful for item %s, order %s", itemId, orderID)
	span.AddEvent("Stock reserved")
	// ------------------ END: 核心业务逻辑 ------------------

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Stock reserved"))
}

// releaseStockHandler 模拟释放库存
func releaseStockHandler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	ctx, span := tracer.Start(ctx, "inventory-service.ReleaseStock (Compensation)")
	defer span.End()

	itemId := r.URL.Query().Get("itemId")
	orderID := r.URL.Query().Get("orderId")

	span.SetAttributes(
		attribute.String("item.id", itemId),
		attribute.String("order.id", orderID),
		attribute.Bool("compensation.logic", true),
	)

	log.Printf("Stock release successful for item %s, order %s", itemId, orderID)
	span.AddEvent("Stock released")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Stock released"))
}

// checkStockHandler 保持不变，可以作为普通查询接口
func checkStockHandler(w http.ResponseWriter, r *http.Request) {
	propagator := otel.GetTextMapPropagator()
	ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	_, span := tracer.Start(ctx, "inventory-service.CheckStock")
	defer span.End()

	itemId := r.URL.Query().Get("itemId")
	log.Printf("Stock check successful for item %s", itemId)
	span.AddEvent("Stock check successful")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Stock available"))
}

/**
Lock() (获取锁)

  想象一下，现在有三个服务（A, B, C）同时来预占同一个商品（比如 itemId=123）的库存，它们都会调用 Lock()。


   1. 创建自己的节点：
       * 每个服务都会在 ZooKeeper 的 /distributed_locks/item-123/ 目录下尝试创建一个临时的、带顺序的节点。
       * 假设 A, B, C 的创建顺序是 A -> B -> C。它们会分别创建出这样的节点：
           * 服务 A: /distributed_locks/item-123/lock-0000000001
           * 服务 B: /distributed_locks/item-123/lock-0000000002
           * 服务 C: /distributed_locks/item-123/lock-0000000003


   2. 判断谁是第一个：
       * 创建完节点后，每个服务都会去获取 /distributed_locks/item-123/ 目录下的所有子节点，并对它们按序号进行排序。
       * 它们都会得到一个列表：[lock-0000000001, lock-0000000002, lock-0000000003]。
       * 然后，每个服务检查自己创建的节点是不是列表中的第一个（序号最小的）。


   3. 获取锁或等待：
       * 服务 A 发现自己的节点 lock-0000000001 是最小的，所以它成功获取了锁，Lock() 函数返回，它开始执行自己的业务逻辑（扣减库存）。
       * 服务 B 发现自己的节点 lock-0000000002 不是最小的。它不会一直傻等，而是会找到排在它前一个的节点，也就是服务 A 创建的
         lock-0000000001。然后，服务 B 会在这个节点上设置一个监视 (Watch)。
       * 服务 C 也一样，它会监视排在它前面的服务 B 的节点 lock-0000000002。

      这个“监视”机制非常关键，它避免了所有等待者都不断地轮询，造成“惊群效应”。只有前一个节点被删除时，下一个等待者才会被唤醒。


   4. 锁的交接：
       * 当服务 A 完成业务逻辑后，它会调用 Unlock()。
       * Unlock() 的逻辑很简单：就是删除自己创建的那个临时节点 (/distributed_locks/item-123/lock-0000000001)。
       * 一旦这个节点被删除，ZooKeeper 就会通知之前监视着它的服务 B。
       * 服务 B 被唤醒后，会重复第 2 步：再次获取子节点列表，检查自己是不是最小的。这时，列表变成了 [lock-0000000002, lock-0000000003]，服务 B
         发现自己是最小的了，于是它成功获取锁。
       * 这个过程会一直持续下去，直到所有服务都执行完毕。
*/

package main

import (
	"context"
	"encoding/json"
	"jaeger-demo/internal/pkg/mq"
	"jaeger-demo/internal/pkg/session"
	"log"
	"os"
	"strings"

	"github.com/segmentio/kafka-go"
)

const (
	kafkaOrderNotificationTopic = "order-notifications-v1"
	consumerGroupID             = "message-router-group-1"
)

var (
	redisAddr    = getEnv("REDIS_ADDR", "localhost:6379")
	kafkaBrokers = strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")
	sessionMgr   *session.Manager
)

type OrderNotificationEvent struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

func main() {
	sessionMgr = session.NewManager(redisAddr)
	reader := mq.NewKafkaReader(kafkaBrokers, kafkaOrderNotificationTopic, consumerGroupID)
	defer reader.Close()

	log.Println("Message Router started. Waiting for notifications...")

	for {
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("ERROR: could not read message: %v", err)
			continue
		}
		go routeMessage(msg)
	}
}

func routeMessage(msg kafka.Message) {
	var event OrderNotificationEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		log.Printf("ERROR: failed to unmarshal notification: %v", err)
		return
	}

	// 1. 从Redis查询用户所在的网关节点
	gatewayNodeID, err := sessionMgr.GetUserGateway(context.Background(), event.UserID)
	if err != nil {
		log.Printf("ERROR: failed to get session for user %s: %v", event.UserID, err)
		return
	}

	if gatewayNodeID == "" {
		log.Printf("User %s is offline. Message dropped.", event.UserID)
		return
	}

	// 2. 路由消息
	// 在生产环境中，这里会通过RPC或另一个Kafka Topic将消息发给目标Gateway
	log.Printf("Routing message for user %s to gateway %s. Message: '%s'",
		event.UserID, gatewayNodeID, event.Message)

	// TODO: 实现真正的路由逻辑
	// e.g., rpcClient.PushToGateway(gatewayNodeID, event.UserID, event.Message)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

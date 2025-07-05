package main

import (
	"context"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"jaeger-demo/internal/pkg/session"
	"log"
	"net/http"
	"os"
	"sync"
)

var (
	redisAddr  = getEnv("REDIS_ADDR", "localhost:6379")
	sessionMgr *session.Manager
	nodeID     = "push-gateway-" + uuid.New().String()[:8]
	upgrader   = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool { // 简化处理，允许所有跨域
			return true
		},
	}
)

// Hub 维护所有活跃的连接，并负责消息广播
type Hub struct {
	clients    map[string]*Client // 使用UserID作为Key
	register   chan *Client
	unregister chan *Client
	lock       sync.RWMutex
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.lock.Lock()
			h.clients[client.userID] = client
			h.lock.Unlock()
			log.Printf("Client %s registered on node %s", client.userID, nodeID)
		case client := <-h.unregister:
			h.lock.Lock()
			if _, ok := h.clients[client.userID]; ok {
				delete(h.clients, client.userID)
				close(client.send)
			}
			h.lock.Unlock()
			log.Printf("Client %s unregistered.", client.userID)
		}
	}
}

// Client 是一个WebSocket连接的代表
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID string
}

func (c *Client) writePump() {
	// ... (负责将send channel中的消息写入websocket)
}
func (c *Client) readPump() {
	// ... (负责读取心跳等消息)
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// 1. 从URL参数获取UserID
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "userID is required", http.StatusBadRequest)
		return
	}

	// 2. HTTP升级为WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// 3. 创建客户端实例并注册到Hub
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), userID: userID}
	client.hub.register <- client

	// 4. 在Redis中设置会话信息
	err = sessionMgr.SetUserGateway(context.Background(), userID, nodeID)
	if err != nil {
		log.Printf("Failed to set session for user %s: %v", userID, err)
		conn.Close()
		return
	}

	// 5. 启动读写goroutine
	go client.writePump()
	go client.readPump()
}

func main() {
	sessionMgr = session.NewManager(redisAddr)
	hub := newHub()
	go hub.run()

	// TODO: 在这里需要一个机制来接收 message-router 路由过来的消息
	// 比如，每个网关节点可以订阅一个自己专属的Kafka Topic ("push-gateway-node-abc-topic")
	// 当接收到消息后，通过 hub.clients[userID].send <- message 推送
	// 为了简化，本样例省略此步，但这是生产实现的关键

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	log.Printf("Push Gateway (%s) started on :8088", nodeID)
	err := http.ListenAndServe(":8088", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

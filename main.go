package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrade = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]bool)

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 将 HTTP 请求升级为 WebSocket 连接
	conn, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	// 将新连接添加到客户端列表中
	clients[conn] = true

	// 发送连接成功的消息给客户端
	_ = conn.WriteMessage(websocket.TextMessage, []byte("clientID:"+wsRemoteAddrToMd5(conn.RemoteAddr().String())))

	// 监听客户端的消息
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNoStatusReceived) {
				log.Println("WebSocket connection closed by client")
			} else {
				log.Println("Error reading message:", err)
			}
			break
		}
		// 接收客户端发送的消息
		log.Println("Received:", string(message))

		// 群发消息给其他客户端
		for c := range clients {
			if c != conn {
				_ = c.WriteMessage(messageType, message)
			}
		}
	}

	// 客户端断开连接
	delete(clients, conn)

	// 发送断开连接的消息给其他客户端
	for c := range clients {
		if c != conn {
			_ = c.WriteMessage(websocket.TextMessage, []byte("clientID:"+wsRemoteAddrToMd5(conn.RemoteAddr().String())+"==>断开连接"))
		}
	}
}

// md5 生成唯一 clientID
func wsRemoteAddrToMd5(remoteAddr string) string {
	hashed := md5.Sum([]byte(remoteAddr))
	return hex.EncodeToString(hashed[:])
}

// 单发消息给指定客户端
func sendToOneClient(clientID string, message string) {
	for c := range clients {
		if wsRemoteAddrToMd5(c.RemoteAddr().String()) == clientID {
			_ = c.WriteMessage(websocket.TextMessage, []byte(message))
		}
		break
	}
}

// 群发消息给所有客户端
func sendToManyClients(message string) {
	for c := range clients {
		_ = c.WriteMessage(websocket.TextMessage, []byte(message))
	}
}

// 单发api
func apiSendOneHandler(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	sendToOneClient(clientID, "我是单发消息")
	_ = json.NewEncoder(w).Encode("ok")
}

// 群发api
func apiSendManyHandler(w http.ResponseWriter, r *http.Request) {
	sendToManyClients("我是群发消息")
	_ = json.NewEncoder(w).Encode("ok")
}

func main() {
	http.HandleFunc("/echo", handleWebSocket)
	http.HandleFunc("/send-one", apiSendOneHandler)   // 测试单发
	http.HandleFunc("/send-many", apiSendManyHandler) // 测试群发
	log.Println("http://127.0.0.1:8080")
	log.Println("ws://127.0.0.1:8080/echo")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

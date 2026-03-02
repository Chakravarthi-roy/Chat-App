package main

import (
	"encoding/json"
	"fmt"
	"log"
	"mychat/login"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================
// WEBSOCKET SETUP
// ============================================

type Message struct {
	Type      string   `json:"type"` // "public", "private", "system", "user_list"
	From      string   `json:"from"`
	To        string   `json:"to,omitempty"`
	Content   string   `json:"content"`
	Users     []string `json:"users,omitempty"`
	Timestamp int64    `json:"timestamp"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: In production, validate origin properly
		return true
	},
}

// ============================================
// TYPES
// ============================================

type Client struct {
	conn     *websocket.Conn
	username string
	userID   int
	send     chan []byte // channel for sending messages to this client
}

// ============================================
// GLOBAL STATE
// ============================================

var clients = make(map[*websocket.Conn]*Client)
var mutex sync.Mutex

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
	sendBufferSize = 256
)

// ============================================
// HELPER FUNCTIONS
// ============================================
// Get list of online usernames
func getOnlineUsernames() []string {
	users := make([]string, 0, len(clients))
	for _, c := range clients {
		users = append(users, c.username)
	}
	return users
}

// Send message to a specific user
func sendToUser(username string, msg Message) bool {
	for _, c := range clients {
		if c.username == username {
			data, err := json.Marshal(msg)
			if err != nil {
				return false
			}
			select {
			case c.send <- data:
				return true
			default:
				return false
			}
		}
	}
	return false
}

// Broadcast message to all users except sender
func broadcastMessage(msg Message, excludeUsername string) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	for _, c := range clients {
		if c.username != excludeUsername {
			select {
			case c.send <- data:
			default:
				log.Printf("Client %s send buffer full", c.username)
			}
		}
	}
}

// Send user list to all clients
func broadcastUserList() {
	users := getOnlineUsernames()
	msg := Message{
		Type:      "user_list",
		Users:     users,
		Timestamp: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for _, c := range clients {
		select {
		case c.send <- data:
		default:
		}
	}
}

// ============================================
// CLIENT WRITE PUMP (Async sender)
// ============================================

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Write error to %s: %v", c.username, err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Broadcast system message to all clients (optionally exclude one)
func broadcastSystemMessage(content string, excludeUsername string) {
	msg := Message{
		Type:      "system",
		Content:   content,
		Timestamp: time.Now().UnixMilli(),
	}

	mutex.Lock()
	defer mutex.Unlock()

	broadcastMessage(msg, excludeUsername)
}

// Get list of online usernames
func getOnlineUsers() []string {
	mutex.Lock()
	defer mutex.Unlock()

	users := make([]string, 0, len(clients))
	for _, c := range clients {
		users = append(users, c.username)
	}
	return users
}

// ============================================
// WEBSOCKET HANDLER
// ============================================

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP to WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer ws.Close()

	// Read username as first message
	_, usernameBytes, err := ws.ReadMessage()
	if err != nil {
		log.Printf("Error reading username: %v", err)
		return
	}

	username := strings.TrimSpace(string(usernameBytes))

	// Validate username
	if username == "" || username == "undefined" || username == "null" {
		log.Printf("Invalid username received: '%s'", username)
		ws.WriteMessage(websocket.TextMessage, []byte("[System] Error: Invalid username"))
		return
	}

	// Create client and add to map
	client := &Client{
		conn:     ws,
		username: username,
		userID:   0,
		send:     make(chan []byte, sendBufferSize),
	}

	mutex.Lock()
	clients[ws] = client
	mutex.Unlock()

	go client.writePump()

	log.Printf("Client connected: %s (%s)", client.username, ws.RemoteAddr())

	// Send current user list to the new client
	mutex.Lock()
	users := getOnlineUsernames()
	mutex.Unlock()

	welcomeMsg := Message{
		Type:      "user_list",
		Users:     users,
		Timestamp: time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(welcomeMsg)
	client.send <- data

	// Notify others that user joined
	broadcastSystemMessage(fmt.Sprintf("%s joined the chat", username), username)

	// Message reading loop
	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Read error from %s: %v", client.username, err)
			break
		}

		messageText := strings.TrimSpace(string(p))

		// Skip empty messages
		if messageText == "" {
			continue
		}

		log.Printf("Message from %s: %s", client.username, messageText)

		// Check if it's a private message (@username message)
		if strings.HasPrefix(messageText, "@") {
			parts := strings.SplitN(messageText, " ", 2)
			if len(parts) == 2 {
				targetUser := strings.TrimPrefix(parts[0], "@")
				privateContent := parts[1]

				privateMsg := Message{
					Type:      "private",
					From:      client.username,
					To:        targetUser,
					Content:   privateContent,
					Timestamp: time.Now().UnixMilli(),
				}

				// Send to target user
				mutex.Lock()
				sent := sendToUser(targetUser, privateMsg)
				mutex.Unlock()

				// Also send back to sender (so they see their own message)
				privateMsg.Type = "private_sent"
				data, _ := json.Marshal(privateMsg)
				client.send <- data

				if !sent {
					// User not found or offline
					errorMsg := Message{
						Type:      "system",
						Content:   fmt.Sprintf("User '%s' is not online", targetUser),
						Timestamp: time.Now().UnixMilli(),
					}
					data, _ := json.Marshal(errorMsg)
					client.send <- data
				}

				continue
			}
		}

		// Public message - broadcast to all
		publicMsg := Message{
			Type:      "public",
			From:      client.username,
			Content:   messageText,
			Timestamp: time.Now().UnixMilli(),
		}

		mutex.Lock()
		broadcastMessage(publicMsg, client.username)
		mutex.Unlock()
	}

	// Cleanup on disconnect
	mutex.Lock()
	delete(clients, ws)
	close(client.send)
	mutex.Unlock()

	log.Printf("Client disconnected: %s (%s)", client.username, ws.RemoteAddr())

	// Notify others that user left
	broadcastSystemMessage(fmt.Sprintf("%s left the chat", username), "")

	// Broadcast updated user list
	mutex.Lock()
	broadcastUserList()
	mutex.Unlock()
}

// ============================================
// HTTP HANDLERS
// ============================================

func onlineUsersHandler(w http.ResponseWriter, r *http.Request) {
	users := getOnlineUsers()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"users": %q, "count": %d}`, users, len(users))
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Test endpoint works!")
}

// ============================================
// MAIN
// ============================================

func main() {
	// WebSocket endpoint
	http.HandleFunc("/ws", handleConnections)

	// Auth endpoints
	http.HandleFunc("/login", login.LoginHandler)
	http.HandleFunc("/register", login.RegisterHandler)

	// Utility endpoints
	http.HandleFunc("/online", onlineUsersHandler)
	http.HandleFunc("/test", testHandler)

	// Static file server
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)

	// Startup message
	fmt.Println("╔═══════════════════════════════════════╗")
	fmt.Println("║     WebSocket Chat Server Started     ║")
	fmt.Println("║                                       ║")
	fmt.Println("║   Local: http://localhost:8080        ║")
	fmt.Println("║                                       ║")
	fmt.Println("║   Endpoints:                          ║")
	fmt.Println("║   • /login.html  - Login page         ║")
	fmt.Println("║   • /register.html - Register page    ║")
	fmt.Println("║   • /main.html  - Chat room           ║")
	fmt.Println("║   • /ws         - WebSocket           ║")
	fmt.Println("║   • /online     - Online users        ║")
	fmt.Println("╚═══════════════════════════════════════╝")

	// Start server
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

package main

import (
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
func broadcastSystemMessage(message string, exclude *websocket.Conn) {
	mutex.Lock()
	defer mutex.Unlock()

	fullMessage := []byte("[System] " + message)
	for _, c := range clients {
		if c.conn != exclude {
			select {
			case c.send <- fullMessage:
			default:
				log.Printf("Client %s send buffer full, skipping", c.username)
			}
		}
	}
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

	// Notify others that user joined
	broadcastSystemMessage(fmt.Sprintf("%s joined the chat", username), ws)

	// Message reading loop
	for {
		_, p, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Read error from %s: %v", client.username, err)
			break
		}

		message := strings.TrimSpace(string(p))

		// Skip empty messages
		if message == "" {
			continue
		}

		log.Printf("Message from %s: %s", client.username, message)

		// Format message with username
		formattedMessage := fmt.Sprintf("%s: %s", client.username, message)

		// Broadcast to all except sender
		mutex.Lock()
		for _, c := range clients {
			if c.conn != ws {
				select {
				case c.send <- []byte(formattedMessage):
				default:
					log.Printf("Client %s send buffer full, dropping message", c.username)
				}
			}
		}
		mutex.Unlock()
	}

	// Cleanup on disconnect
	mutex.Lock()
	delete(clients, ws)
	close(client.send)
	mutex.Unlock()

	log.Printf("Client disconnected: %s (%s)", client.username, ws.RemoteAddr())

	// Notify others that user left
	broadcastSystemMessage(fmt.Sprintf("%s left the chat", username), nil)
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

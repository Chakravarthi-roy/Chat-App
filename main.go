package main

import (
	"fmt"
	"log"
	"mychat/login"
	"net/http"
	"strings"
	"sync"

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
}

// ============================================
// GLOBAL STATE
// ============================================

var clients = make(map[*websocket.Conn]*Client)
var mutex sync.Mutex

// ============================================
// HELPER FUNCTIONS
// ============================================

// Broadcast system message to all clients (optionally exclude one)
func broadcastSystemMessage(message string, exclude *websocket.Conn) {
	mutex.Lock()
	defer mutex.Unlock()

	for _, c := range clients {
		if c.conn != exclude {
			c.conn.WriteMessage(websocket.TextMessage, []byte("[System] "+message))
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
	}

	mutex.Lock()
	clients[ws] = client
	mutex.Unlock()

	log.Printf("Client connected: %s (%s)", client.username, ws.RemoteAddr())

	// Notify others that user joined
	broadcastSystemMessage(fmt.Sprintf("%s joined the chat", username), ws)

	// Message reading loop
	for {
		mt, p, err := ws.ReadMessage()
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
				err = c.conn.WriteMessage(mt, []byte(formattedMessage))
				if err != nil {
					log.Printf("Write error to %s: %v", c.username, err)
					c.conn.Close()
					delete(clients, c.conn)
				}
			}
		}
		mutex.Unlock()
	}

	// Cleanup on disconnect
	mutex.Lock()
	delete(clients, ws)
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

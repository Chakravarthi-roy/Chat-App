package main

import (
	"fmt"
	"log"
	"mychat/login"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	conn     *websocket.Conn
	username string
	userID   int
}

var clients = make(map[*websocket.Conn]*Client)
var mutex sync.Mutex

func handleConncetions(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil) //upgrades the initial get request to a websocket connection
	if err != nil {
		log.Fatal(err)
	}

	defer ws.Close()

	//
	_, usernameBytes, err := ws.ReadMessage()
	if err != nil {
		log.Printf("error reading username: %v", err)
		return
	}
	username := string(usernameBytes)
	//

	client := &Client{conn: ws, username: username, userID: 0} //Default username

	mutex.Lock()
	clients[ws] = client
	mutex.Unlock()

	fmt.Printf("Client connected:%s (%s)\n", ws.RemoteAddr(), client.username)

	// read the message from the websocket
	for {
		mt, p, err := ws.ReadMessage()
		if err != nil {
			log.Printf("read error from %s: %v", client.username, err)
			mutex.Lock()
			delete(clients, ws)
			mutex.Unlock()
			fmt.Printf("Client disconnected:%s (%s)\n", ws.RemoteAddr(), client.username)
			break
		}

		log.Printf("recv: %s from %s (%s)", p, ws.RemoteAddr(), client.username)

		//formatting the message with username
		formattedMessage := fmt.Sprintf("%s: %s", client.username, p)

		// echo back the same message
		//err = ws.WriteMessage(mt, p)
		//if err != nil {
		//	log.Println("write:", err)
		//	break
		//}

		//broadcast message to all connected clients except to itself
		mutex.Lock()
		for _, c := range clients {
			if c.conn != ws {
				err = c.conn.WriteMessage(mt, []byte(formattedMessage))
				if err != nil {
					log.Printf("write error to %s (%s): %v", c.conn.RemoteAddr(), c.username, err)

					delete(clients, c.conn)
				}
			}
		}
		mutex.Unlock()
	}
}

func main() {
	http.HandleFunc("/ws", handleConncetions)
	http.HandleFunc("/login", login.LoginHandler)
	http.HandleFunc("/register", login.RegisterHandler)
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Test Endpoint works")
	})

	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)

	fmt.Println("WebSocket server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

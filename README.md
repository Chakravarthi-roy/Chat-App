# Real-Time Chat Application

A real-time chat app built with Go and WebSockets. I built this to understand how persistent connections, message broadcasting, and real-time communication actually work under the hood, not just use a library, but really get it.

Currently, it handles **500 concurrent users**, **37,000+ messages per second**, and **sub-200ms latency** with a 99% message delivery rate under load.

Built with **Go**, **PostgreSQL**, and **JavaScript**.

---

## Why I Built This

I wanted to learn Go and understand WebSockets from first principles. I also have a feature idea I want to build eventually and this chat app is the foundation for that.

---

## How It Works

The server maintains a persistent WebSocket connection with each user. When someone sends a message, the server instantly broadcasts it to all connected clients (for public messages) or routes it to a specific user (for private messages).

```
┌──────────┐   WebSocket   ┌──────────────┐
│  User A  │◄─────────────►│              │
└──────────┘               │  Go Server   │
                           │              │
┌──────────┐   WebSocket   │              │
│  User B  │◄─────────────►│              │
└──────────┘               └──────┬───────┘
                                  │
                                  ▼
                           ┌──────────────┐
                           │  PostgreSQL  │
                           │   (users)    │
                           └──────────────┘
```

**Features:**
- User registration and login
- Real-time public chat
- Private messaging
- Online users list in sidebar
- Unread message indicators

---

## Project Structure

```
mychat/
├── main.go               # Server entry point, HTTP routes
├── hub.go                # Connection manager, message broadcasting
├── client.go             # Individual client handler, read/write pumps
├── login/
│   └── login.go          # Authentication, PostgreSQL connection
├── main.html             # Chat interface
├── main.js               # Frontend WebSocket logic
├── main.css              # Styling
├── login.html            # Login page
└── register.html         # Registration page
```

---

## Running Locally

**Prerequisites**
- Go 1.21 or higher
- PostgreSQL

**1. Clone the repository**
```bash
git clone https://github.com/Chakravarthi-roy/mychat.git
cd mychat
```

**2. Set up PostgreSQL**

Create a database named `test2` (or update it in `login/login.go`):
```sql
CREATE DATABASE test2;
```
The users table is created automatically when you run the server.

**3. Install Go dependencies**
```bash
go mod tidy
```

**4. Run the server**
```bash
go run main.go hub.go client.go
```

**5. Open in browser**

Go to `http://localhost:8080/register.html` to create an account, then login and start chatting.

---

## Load Testing

Tested with **k6**:

| Metric | Result |
|---|---|
| Concurrent Connections | 500 |
| Messages per Second | 37,000+ |
| Connection Success Rate | 100% |
| Message Latency | sub-200ms |
| Message Delivery Rate | 99% under load |

**Run the tests yourself:**
```bash
# Install k6
brew install k6        # Mac
choco install k6       # Windows

# Run
cd loadtest
k6 run websocket_test.js
```
OR (for windows)
1. Go to [k6 GitHub Releases](https://github.com/grafana/k6/releases)
2. Download the latest `k6-vX.XX.X-windows-amd64.zip`
3. Unzip the file
4. Move `k6.exe` to a folder (e.g., `C:\k6\`)
5. Add to PATH:
   - Press `Windows + S` and search "Environment Variables"
   - Click "Edit the system environment variables"
   - Click "Environment Variables" button
   - Under "System variables", find `Path` and click "Edit"
   - Click "New" and add `C:\k6\`
   - Click "OK" on all windows
6. Open a new terminal and verify:
```bash
k6 version
```
---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go, Gorilla WebSocket |
| Database | PostgreSQL |
| Frontend | HTML, CSS, Vanilla JavaScript |
| Containerization | Docker |
| Load Testing | k6 |

---

## What's Next

Currently scaling the architecture to handle 10,000+ concurrent users:

- Sharded connection management (32 shards) to reduce lock contention
- Worker pool for parallel message broadcasting (16 workers)
- Redis Pub/Sub for horizontal scaling across multiple servers
- CI/CD pipeline with automated deployment

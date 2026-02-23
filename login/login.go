package login

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// ============================================
// TYPES
// ============================================

type User struct {
	ID        int
	Username  string
	HPassword string
}

// Response struct for consistent JSON responses
type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	UserID  int    `json:"userID,omitempty"`
}

var DB *sql.DB

// ============================================
// HELPER FUNCTIONS
// ============================================

// Get environment variable with fallback default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Send JSON response with proper headers
func sendJSON(w http.ResponseWriter, statusCode int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// ============================================
// DATABASE
// ============================================

func InitializeDB() error {
	// Use environment variables with fallbacks
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "123456789")
	dbname := getEnv("DB_NAME", "test2")

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify connection actually works
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db

	// Create users table
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	log.Println("Database initialized successfully")
	return nil
}

// ============================================
// REGISTRATION
// ============================================

func RegisterUser(username, password string) error {
	// Server-side validation
	if len(username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = DB.Exec("INSERT INTO users (username, password) VALUES ($1, $2)", username, hashedPassword)
	if err != nil {
		return fmt.Errorf("username already exists")
	}

	return nil
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSON(w, http.StatusMethodNotAllowed, Response{
			Status:  "error",
			Message: "Method not allowed",
		})
		return
	}

	err := r.ParseForm()
	if err != nil {
		sendJSON(w, http.StatusBadRequest, Response{
			Status:  "error",
			Message: "Error parsing form",
		})
		return
	}

	username := r.Form.Get("username")
	password := r.Form.Get("password")

	// Check empty values
	if username == "" || password == "" {
		sendJSON(w, http.StatusBadRequest, Response{
			Status:  "error",
			Message: "Username and password are required",
		})
		return
	}

	err = RegisterUser(username, password)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, Response{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	log.Printf("New user registered: %s", username)
	sendJSON(w, http.StatusOK, Response{
		Status:  "success",
		Message: "Registration successful",
	})
}

// ============================================
// LOGIN / AUTHENTICATION
// ============================================

func AuthenticateUser(username, password string) (*User, error) {
	query := "SELECT id, username, password FROM users WHERE username = $1"
	row := DB.QueryRow(query, username)

	var user User
	err := row.Scan(&user.ID, &user.Username, &user.HPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid username or password")
		}
		return nil, fmt.Errorf("database error")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.HPassword), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	return &user, nil
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSON(w, http.StatusMethodNotAllowed, Response{
			Status:  "error",
			Message: "Method not allowed",
		})
		return
	}

	err := r.ParseForm()
	if err != nil {
		sendJSON(w, http.StatusBadRequest, Response{
			Status:  "error",
			Message: "Error parsing form",
		})
		return
	}

	username := r.Form.Get("username")
	password := r.Form.Get("password")

	// Check empty values
	if username == "" || password == "" {
		sendJSON(w, http.StatusBadRequest, Response{
			Status:  "error",
			Message: "Username and password are required",
		})
		return
	}

	user, err := AuthenticateUser(username, password)
	if err != nil {
		log.Printf("Failed login attempt for user: %s", username)
		sendJSON(w, http.StatusUnauthorized, Response{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	log.Printf("User logged in: %s (ID: %d)", user.Username, user.ID)
	sendJSON(w, http.StatusOK, Response{
		Status:  "success",
		Message: "Login successful",
		UserID:  user.ID,
	})
}

// ============================================
// INITIALIZATION
// ============================================

func init() {
	if err := InitializeDB(); err != nil {
		// Fail loudly - don't let app run with broken DB
		log.Fatalf("FATAL: Error initializing database: %v", err)
	}
}

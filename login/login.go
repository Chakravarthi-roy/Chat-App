package login

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int
	Username  string
	HPassword string
}

var DB *sql.DB

type LoginForm struct {
	Error string
}

func InitializeDB() error {
	connStr := "host=localhost dbname=test2 port=5432 user=postgres password=123456789 sslmode=disable"

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	DB = db

	//creating users table
	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	return nil
}

// creates a new database

// registration handlers!!
func RegisterUser(username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	_, err = DB.Exec("INSERT INTO users (username, password) VALUES ($1, $2)", username, hashedPassword) //main exec
	//(?, ?) - sqlite
	if err != nil {
		return fmt.Errorf("failed to register user: %w", err)
	}

	return nil
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	username := r.Form.Get("username")
	password := r.Form.Get("password")

	err = RegisterUser(username, password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, `{"status": "success", "message": "Registration successful"}`)
}

// login handlers!! with commented log statements inorder to pinpoint the errors
func AuthenticateUser(username, password string) (*User, error) {
	// log.Println("authenticate user:", username)
	query := "SELECT id, username, password FROM users WHERE username = $1"
	// log.Println("exec query:", query)
	row := DB.QueryRow(query, username)

	var user User
	err := row.Scan(&user.ID, &user.Username, &user.HPassword)
	if err != nil {
		// log.Println("Error scaning:", err)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid username or password")
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	// log.Println("retrieved user:", user)
	err = bcrypt.CompareHashAndPassword([]byte(user.HPassword), []byte(password))
	if err != nil {
		// log.Println("error hash comparing:", err)
		return nil, fmt.Errorf("invalid username or password")
	}

	return &user, nil
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	username := r.Form.Get("username")
	password := r.Form.Get("password")

	user, err := AuthenticateUser(username, password)
	if err != nil {
		log.Println("Error authenticating:", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	fmt.Fprintf(w, `{"status": "success", "message": "Login", "userID": %d}`, user.ID)
}

func init() {
	if err := InitializeDB(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
	}
}

package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	ShortURL string `json:"short_url"`
}

type GetURLResponse struct {
	URL string `json:"url"`
}

const (
	codeLength = 6
	charset    = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNOPQRSTUVWXYZ0123456789"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	db, err := sqlx.Connect("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}

	r := mux.NewRouter()

	r.HandleFunc("/shorten", ShortenURLHandler(db)).Methods("POST")
	r.HandleFunc("/stats/{code}", GetURLStatsHandler(db)).Methods("GET")
	r.HandleFunc("/link/{code}", GetURLHandler(db)).Methods("GET")

	log.Println("Server started on http://localhost:3000")
	log.Fatal(http.ListenAndServe(":3000", r))
}

func ShortenURLHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request ShortenRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var existingCode string
		query := `SELECT code FROM links WHERE = $1`
		err = db.Get(&existingCode, query, request.URL)
		if err == nil {
			response := ShortenResponse{
				ShortURL: existingCode,
			}

			jsonResponse, err := json.Marshal(response)
			if err != nil {
				log.Println("Error marshaling JSON response:", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(jsonResponse)
			return
		} else if err != sql.ErrNoRows {
			log.Println("Error querying database:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		code := generateCode()

		query = `INSERT INTO links (code, url, created_at) VALUES ($1, $2, $3)`
		_, err = db.Exec(query, code, request.URL, time.Now())
		if err != nil {
			log.Println("Error inserting URL into the database:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		response := ShortenResponse{
			ShortURL: code,
		}

		jsonResponse, err := json.Marshal(response)
		if err != nil {
			log.Println("Error marshaling JSON response:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}
}

func GetURLStatsHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Retrieve the short code from the URL path parameters
		// Query the database using the provided connection to get the stats for the given short code

		// Return the stats data in the response
	}
}

func GetURLHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		code := vars["code"]

		query := `SELECT url FROM links WHERE code = $1`
		var url string
		err := db.Get(&url, query, code)
		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
			} else {
				log.Println("Error querying database:", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		response := GetURLResponse{
			URL: url,
		}

		jsonResponse, err := json.Marshal(response)
		if err != nil {
			log.Println("Error marshaling JSON response:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
	}
}

func generateCode() string {
	rand.Seed(time.Now().UnixNano())

	code := make([]byte, codeLength)
	for i := 0; i < codeLength; i++ {
		code[i] = charset[rand.Intn(len(charset))]
	}

	return string(code)
}

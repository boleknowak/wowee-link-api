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

type IndexResponse struct {
	Status string `json:"status"`
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	ShortURL string `json:"short_url"`
}

type GetURLResponse struct {
	URL string `json:"url"`
}

type Link struct {
	ID           int       `db:"id" json:"id"`
	Code         string    `db:"code" json:"code"`
	URL          string    `db:"url" json:"url"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	AttemptCount int       `db:"attempt_count" json:"attempt_count"`
	ClickCount   int       `db:"click_count" json:"click_count"`
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

	r.HandleFunc("/", IndexURLHandler(db)).Methods("GET")
	r.HandleFunc("/shorten", ShortenURLHandler(db)).Methods("POST")
	r.HandleFunc("/stats/{code}", GetURLStatsHandler(db)).Methods("GET")
	r.HandleFunc("/get-link/{code}", GetURLHandler(db)).Methods("GET")

	log.Println("[INFO] Server started on http://localhost:8000")
	log.Fatal(http.ListenAndServe(":8000", r))
}

func IndexURLHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := IndexResponse{
			Status: "OK",
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

func ShortenURLHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request ShortenRequest
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var result struct {
			Code         string
			AttemptCount int `db:"attempt_count"`
		}

		query := `SELECT code, attempt_count FROM links WHERE url = $1`
		err = db.Get(&result, query, request.URL)

		existingCode := result.Code
		attemptCount := result.AttemptCount

		if err == nil {
			response := ShortenResponse{
				ShortURL: existingCode,
			}

			query = `UPDATE links SET attempt_count = $1 WHERE code = $2`
			_, err = db.Exec(query, attemptCount+1, existingCode)
			if err != nil {
				log.Println("Error updating attempt_count in the database:", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
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

		query = `INSERT INTO links (code, url, created_at, attempt_count) VALUES ($1, $2, $3, $4)`
		_, err = db.Exec(query, code, request.URL, time.Now(), 1)
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
		vars := mux.Vars(r)
		code := vars["code"]

		query := `
			SELECT id, code, url, created_at, attempt_count, click_count
			FROM links
			WHERE code = $1
		`
		var link Link
		err := db.Get(&link, query, code)
		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
			} else {
				log.Println("Error querying database:", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		response := Link{
			ID:           link.ID,
			Code:         link.Code,
			URL:          link.URL,
			CreatedAt:    link.CreatedAt,
			AttemptCount: link.AttemptCount,
			ClickCount:   link.ClickCount,
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

func GetURLHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		code := vars["code"]

		query := `SELECT id, url FROM links WHERE code = $1`
		var link Link
		err := db.Get(&link, query, code)

		if err != nil {
			if err == sql.ErrNoRows {
				http.NotFound(w, r)
			} else {
				log.Println("Error querying database:", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
			return
		}

		clickCountQuery := `UPDATE links SET click_count = click_count + 1 WHERE id = $1`
		_, err = db.Exec(clickCountQuery, link.ID)
		if err != nil {
			log.Println("Error updating click count:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}

		clicksQuery := `
    INSERT INTO clicks (link_id, clicks, date)
    VALUES ($1, 1, $2)
    ON CONFLICT (link_id, date)
    DO UPDATE SET clicks = clicks.clicks + 1
`
		_, err = db.Exec(clicksQuery, link.ID, time.Now().UTC().Format("2006-01-02"))
		if err != nil {
			log.Println("Error inserting/updating click count:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}

		response := GetURLResponse{
			URL: link.URL,
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

package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"minion-bank-backend/internal/config"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB(cfg *config.Config) {
	connStr := fmt.Sprintf("host=%s port=5432 user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBUser, cfg.DBPass, cfg.DBName)

	var err error
	for i := 0; i < 5; i++ {
		DB, err = sql.Open("postgres", connStr)
		if err == nil {
			err = DB.Ping()
			if err == nil {
				log.Println("Database connection established")
				break
			}
		}
		log.Printf("DB connection failed (attempt %d/5): %v", i+1, err)
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		log.Fatal("Could not connect to DB")
	}

	createTables()
}

func createTables() {
	query := `CREATE TABLE IF NOT EXISTS card_requests (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		phone VARCHAR(50) NOT NULL,
		email VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := DB.Exec(query); err != nil {
		log.Fatalf("Table creation failed: %v", err)
	}
}

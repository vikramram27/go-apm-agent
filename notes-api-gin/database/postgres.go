package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"go.elastic.co/apm/module/apmsql/v2"
	_ "go.elastic.co/apm/module/apmsql/v2/pq" // APM-instrumented lib/pq driver
	"notes-api-gin/config"
)

func ConnectPostgres(cfg *config.Config) (*sql.DB, error) {
	dsn := cfg.PostgresDSN()
	var db *sql.DB
	var err error

	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		db, err = apmsql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Println("Successfully connected to PostgreSQL (notes-api-gin)")
				break
			}
		}
		log.Printf("PostgreSQL connection attempt %d/%d failed: %v. Retrying in 3s...", attempt, maxAttempts, err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect to PostgreSQL after %d attempts: %w", maxAttempts, err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		username VARCHAR(50) UNIQUE NOT NULL,
		email VARCHAR(100) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS notes (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		title VARCHAR(255) NOT NULL,
		content TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS note_shares (
		id BIGSERIAL PRIMARY KEY,
		note_id BIGINT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
		shared_with_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		permission VARCHAR(20) NOT NULL DEFAULT 'read' CHECK (permission IN ('read', 'write')),
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (note_id, shared_with_user_id)
	);

	CREATE INDEX IF NOT EXISTS idx_notes_user_id ON notes(user_id);
	CREATE INDEX IF NOT EXISTS idx_note_shares_user_id ON note_shares(shared_with_user_id);
	CREATE INDEX IF NOT EXISTS idx_note_shares_note_id ON note_shares(note_id);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}
	log.Println("Database migrations applied successfully (notes-api-gin)")
	return nil
}


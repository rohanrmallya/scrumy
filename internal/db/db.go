package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

//go:embed schema.sql
var schemaSQL string

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", fmt.Sprintf("%s?_foreign_keys=on&_journal_mode=WAL", path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1) // sqlite is single-writer
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := conn.Exec(schemaSQL); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	db := &DB{conn}
	if err := db.SeedRootUser(); err != nil {
		return nil, fmt.Errorf("seed root user: %w", err)
	}
	return db, nil
}

func (db *DB) SeedRootUser() error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'root'").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("root"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	rootID := uuid.New().String()
	_, err = db.Exec("INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)",
		rootID, "root", string(hash), "admin")
	if err != nil {
		return err
	}

	// Link all existing plans to root if they don't have an admin
	rows, err := db.Query("SELECT id FROM plans WHERE id NOT IN (SELECT plan_id FROM plan_admins)")
	if err != nil {
		return err
	}
	defer rows.Close()

	var planIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		planIDs = append(planIDs, id)
	}

	for _, planID := range planIDs {
		_, err = db.Exec("INSERT INTO plan_admins (plan_id, user_id) VALUES (?, ?)", planID, rootID)
		if err != nil {
			return err
		}
	}
	return nil
}

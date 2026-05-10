package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"log"
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

	if err := db.Migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	if err := db.SeedRootUser(); err != nil {
		return nil, fmt.Errorf("seed root user: %w", err)
	}
	return db, nil
}

func (db *DB) Migrate() error {
	// Migration 1: Add template_id to presentations if not exists
	var hasTemplateID bool
	rows, err := db.Query("PRAGMA table_info(presentations)")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dtype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &dtype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "template_id" {
			hasTemplateID = true
			break
		}
	}

	if !hasTemplateID {
		log.Println("⚡ Migrating: adding template_id to presentations table")
		_, err = db.Exec("ALTER TABLE presentations ADD COLUMN template_id TEXT NOT NULL DEFAULT 'default'")
		if err != nil {
			return err
		}
	}
	return nil
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

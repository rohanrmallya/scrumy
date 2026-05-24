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
	for rows.Next() {
		var cid int
		var name, dtype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &dtype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "template_id" {
			hasTemplateID = true
		}
	}
	rows.Close()

	if !hasTemplateID {
		log.Println("⚡ Migrating: adding template_id to presentations table")
		_, err = db.Exec("ALTER TABLE presentations ADD COLUMN template_id TEXT NOT NULL DEFAULT 'default'")
		if err != nil {
			return err
		}
	}

	// Migration 2: Add Jira columns to plans table if not exists
	planColumns, err := db.getTableColumns("plans")
	if err != nil {
		return fmt.Errorf("get plans columns: %w", err)
	}
	newCols := map[string]string{
		"jira_url":      "ALTER TABLE plans ADD COLUMN jira_url TEXT NOT NULL DEFAULT ''",
		"jira_user":     "ALTER TABLE plans ADD COLUMN jira_user TEXT NOT NULL DEFAULT ''",
		"jira_token":    "ALTER TABLE plans ADD COLUMN jira_token TEXT NOT NULL DEFAULT ''",
		"jira_jql":      "ALTER TABLE plans ADD COLUMN jira_jql TEXT NOT NULL DEFAULT ''",
		"jira_sp_field": "ALTER TABLE plans ADD COLUMN jira_sp_field TEXT NOT NULL DEFAULT ''",
		"jira_insecure": "ALTER TABLE plans ADD COLUMN jira_insecure INTEGER NOT NULL DEFAULT 0",
	}
	for col, sqlStmt := range newCols {
		found := false
		for _, c := range planColumns {
			if c == col {
				found = true
				break
			}
		}
		if !found {
			log.Printf("⚡ Migrating: adding %s to plans table\n", col)
			if _, err := db.Exec(sqlStmt); err != nil {
				return fmt.Errorf("add column %s: %w", col, err)
			}
		}
	}

	// Migration 3: Create jira_snapshots table and index if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jira_snapshots (
			id          TEXT PRIMARY KEY,
			plan_id     TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
			name        TEXT NOT NULL,
			start_date  TEXT NOT NULL, -- YYYY-MM-DD
			end_date    TEXT NOT NULL, -- YYYY-MM-DD
			data        TEXT NOT NULL DEFAULT '{}', -- JSON blob
			created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create jira_snapshots table: %w", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_jira_snapshots_plan_id ON jira_snapshots(plan_id)`)
	if err != nil {
		return fmt.Errorf("create index idx_jira_snapshots_plan_id: %w", err)
	}

	return nil
}

func (db *DB) getTableColumns(table string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var cid int
		var name, dtype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &dtype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, nil
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

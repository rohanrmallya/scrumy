package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
	"log"
	"os"
	"strings"
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
	// Preamble: schema.sql pre-creates empty teams/team_admins tables even on
	// pre-rename databases. Remove those empty shells when legacy tables exist
	// so subsequent migrations (and the Migration 7 RENAME, which preserves
	// foreign-key references) operate on the real data.
	if err := dropEmptyTeamShells(db); err != nil {
		return err
	}

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

	// Migration 2: Add Jira columns to teams table if not exists
	// (pre-rename databases carry this table as "plans"; Migration 7 renames it)
	teamTable := "teams"
	if !tableExists(db, "teams") && tableExists(db, "plans") {
		teamTable = "plans"
	}
	planColumns, err := db.getTableColumns(teamTable)
	if err != nil {
		return fmt.Errorf("get teams columns: %w", err)
	}
	newCols := map[string]string{
		"jira_url":      "ALTER TABLE " + teamTable + " ADD COLUMN jira_url TEXT NOT NULL DEFAULT ''",
		"jira_user":     "ALTER TABLE " + teamTable + " ADD COLUMN jira_user TEXT NOT NULL DEFAULT ''",
		"jira_token":    "ALTER TABLE " + teamTable + " ADD COLUMN jira_token TEXT NOT NULL DEFAULT ''",
		"jira_jql":      "ALTER TABLE " + teamTable + " ADD COLUMN jira_jql TEXT NOT NULL DEFAULT ''",
		"jira_sp_field": "ALTER TABLE " + teamTable + " ADD COLUMN jira_sp_field TEXT NOT NULL DEFAULT ''",
		"jira_insecure": "ALTER TABLE " + teamTable + " ADD COLUMN jira_insecure INTEGER NOT NULL DEFAULT 0",
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
			log.Printf("⚡ Migrating: adding %s to %s table\n", col, teamTable)
			if _, err := db.Exec(sqlStmt); err != nil {
				return fmt.Errorf("add column %s: %w", col, err)
			}
		}
	}

	// Migration 3: Create jira_snapshots table and index if not exists
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jira_snapshots (
			id            TEXT PRIMARY KEY,
			team_id       TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			name          TEXT NOT NULL,
			start_date    TEXT NOT NULL, -- YYYY-MM-DD
			end_date      TEXT NOT NULL, -- YYYY-MM-DD
			all_worklogs  INTEGER NOT NULL DEFAULT 0,
			data          TEXT NOT NULL DEFAULT '{}', -- JSON blob
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create jira_snapshots table: %w", err)
	}
	// Index the owning-team column under whichever name this database uses;
	// Migration 7 normalizes everything to team_id.
	if columnExists(db, "jira_snapshots", "team_id") {
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_jira_snapshots_team_id ON jira_snapshots(team_id)`)
		if err != nil {
			return fmt.Errorf("create index idx_jira_snapshots_team_id: %w", err)
		}
	} else {
		_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_jira_snapshots_plan_id ON jira_snapshots(plan_id)`)
		if err != nil {
			return fmt.Errorf("create index idx_jira_snapshots_plan_id: %w", err)
		}
	}

	// Migration 4: Add all_worklogs to jira_snapshots if not exists
	snapshotColumns, err := db.getTableColumns("jira_snapshots")
	if err != nil {
		return fmt.Errorf("get jira_snapshots columns: %w", err)
	}
	hasAllWorklogs := false
	for _, c := range snapshotColumns {
		if c == "all_worklogs" {
			hasAllWorklogs = true
			break
		}
	}
	if !hasAllWorklogs {
		log.Println("⚡ Migrating: adding all_worklogs to jira_snapshots table")
		_, err = db.Exec("ALTER TABLE jira_snapshots ADD COLUMN all_worklogs INTEGER NOT NULL DEFAULT 0")
		if err != nil {
			return fmt.Errorf("add all_worklogs column: %w", err)
		}
	}

	// Migration 5: Create jira_snapshot_refresh_logs table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jira_snapshot_refresh_logs (
			id                  TEXT PRIMARY KEY,
			snapshot_id         TEXT NOT NULL REFERENCES jira_snapshots(id) ON DELETE CASCADE,
			refreshed_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
			prev_story_points   REAL NOT NULL,
			new_story_points    REAL NOT NULL,
			prev_hours_logged   REAL NOT NULL,
			new_hours_logged    REAL NOT NULL,
			prev_worklogs_count INTEGER NOT NULL,
			new_worklogs_count  INTEGER NOT NULL,
			prev_issues_count   INTEGER NOT NULL,
			new_issues_count    INTEGER NOT NULL,
			user_deltas         TEXT NOT NULL DEFAULT '[]',
			user_deltas_all     TEXT NOT NULL DEFAULT '[]'
		)
	`)
	if err != nil {
		return fmt.Errorf("create jira_snapshot_refresh_logs table: %w", err)
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_jira_snapshot_refresh_logs_snapshot_id ON jira_snapshot_refresh_logs(snapshot_id)`)
	if err != nil {
		return fmt.Errorf("create index idx_jira_snapshot_refresh_logs_snapshot_id: %w", err)
	}

	// Migration 6: Add user_deltas and user_deltas_all to jira_snapshot_refresh_logs if not exists
	refreshLogColumns, err := db.getTableColumns("jira_snapshot_refresh_logs")
	if err != nil {
		return fmt.Errorf("get jira_snapshot_refresh_logs columns: %w", err)
	}
	hasUserDeltas := false
	hasUserDeltasAll := false
	for _, c := range refreshLogColumns {
		if c == "user_deltas" {
			hasUserDeltas = true
		}
		if c == "user_deltas_all" {
			hasUserDeltasAll = true
		}
	}
	if !hasUserDeltas {
		log.Println("⚡ Migrating: adding user_deltas to jira_snapshot_refresh_logs table")
		_, err = db.Exec("ALTER TABLE jira_snapshot_refresh_logs ADD COLUMN user_deltas TEXT NOT NULL DEFAULT '[]'")
		if err != nil {
			return fmt.Errorf("add user_deltas column: %w", err)
		}
	}
	if !hasUserDeltasAll {
		log.Println("⚡ Migrating: adding user_deltas_all to jira_snapshot_refresh_logs table")
		_, err = db.Exec("ALTER TABLE jira_snapshot_refresh_logs ADD COLUMN user_deltas_all TEXT NOT NULL DEFAULT '[]'")
		if err != nil {
			return fmt.Errorf("add user_deltas_all column: %w", err)
		}
	}

	// Migration 7: Rename plans -> teams, plan_admins -> team_admins, plan_id -> team_id
	if err := migratePlansToTeams(db); err != nil {
		return fmt.Errorf("migrate plans to teams: %w", err)
	}

	// Migration 8: Add auto-refresh columns to jira_snapshots and create jira_auto_refresh_audits table
	autoRefreshCols := map[string]string{
		"auto_refresh_enabled":     "ALTER TABLE jira_snapshots ADD COLUMN auto_refresh_enabled INTEGER NOT NULL DEFAULT 0",
		"auto_refresh_time":        "ALTER TABLE jira_snapshots ADD COLUMN auto_refresh_time TEXT NOT NULL DEFAULT '09:00'",
		"next_run_at":              "ALTER TABLE jira_snapshots ADD COLUMN next_run_at DATETIME",
		"last_auto_refreshed_at":   "ALTER TABLE jira_snapshots ADD COLUMN last_auto_refreshed_at DATETIME",
		"last_auto_refresh_status": "ALTER TABLE jira_snapshots ADD COLUMN last_auto_refresh_status TEXT NOT NULL DEFAULT ''",
		"last_auto_refresh_msg":    "ALTER TABLE jira_snapshots ADD COLUMN last_auto_refresh_msg TEXT NOT NULL DEFAULT ''",
	}
	for col, sqlStmt := range autoRefreshCols {
		if !columnExists(db, "jira_snapshots", col) {
			log.Printf("⚡ Migrating: adding %s to jira_snapshots table\n", col)
			if _, err := db.Exec(sqlStmt); err != nil {
				return fmt.Errorf("add column %s to jira_snapshots: %w", col, err)
			}
		}
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jira_auto_refresh_audits (
			id                  TEXT PRIMARY KEY,
			snapshot_id         TEXT NOT NULL REFERENCES jira_snapshots(id) ON DELETE CASCADE,
			team_id             TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			triggered_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
			trigger_type        TEXT NOT NULL DEFAULT 'scheduled',
			status              TEXT NOT NULL,
			message             TEXT NOT NULL DEFAULT '',
			prev_story_points   REAL NOT NULL DEFAULT 0,
			new_story_points    REAL NOT NULL DEFAULT 0,
			prev_hours_logged   REAL NOT NULL DEFAULT 0,
			new_hours_logged    REAL NOT NULL DEFAULT 0,
			prev_issues_count   INTEGER NOT NULL DEFAULT 0,
			new_issues_count    INTEGER NOT NULL DEFAULT 0,
			duration_ms         INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("create jira_auto_refresh_audits table: %w", err)
	}

	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_jira_auto_refresh_audits_snapshot_id ON jira_auto_refresh_audits(snapshot_id)",
		"CREATE INDEX IF NOT EXISTS idx_jira_auto_refresh_audits_team_id ON jira_auto_refresh_audits(team_id)",
		"CREATE INDEX IF NOT EXISTS idx_jira_auto_refresh_audits_triggered_at ON jira_auto_refresh_audits(triggered_at)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("create auto refresh audit index: %w", err)
		}
	}

	return nil
}

// tableExists reports whether a table exists in the database.
func tableExists(db *DB, table string) bool {
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
	return err == nil && name == table
}

// columnExists reports whether a column exists on a table.
func columnExists(db *DB, table, column string) bool {
	cols, err := db.getTableColumns(table)
	if err != nil {
		return false
	}
	for _, c := range cols {
		if c == column {
			return true
		}
	}
	return false
}

// isTableEmpty reports whether a table holds zero rows.
func isTableEmpty(db *DB, table string) bool {
	var count int
	if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
		return false
	}
	return count == 0
}

// dropEmptyTeamShells removes empty teams/team_admins tables pre-created by
// schema.sql when the legacy plans/plan_admins tables hold the real data.
// This lets Migration 7 use RENAME (which keeps FK references intact).
func dropEmptyTeamShells(db *DB) error {
	if tableExists(db, "plans") && tableExists(db, "teams") && isTableEmpty(db, "teams") {
		log.Println("⚡ Migrating: dropping empty teams shell pre-created by schema")
		if _, err := db.Exec("DROP TABLE teams"); err != nil {
			return fmt.Errorf("drop empty teams shell: %w", err)
		}
	}
	if tableExists(db, "plan_admins") && tableExists(db, "team_admins") && isTableEmpty(db, "team_admins") {
		log.Println("⚡ Migrating: dropping empty team_admins shell pre-created by schema")
		if _, err := db.Exec("DROP TABLE team_admins"); err != nil {
			return fmt.Errorf("drop empty team_admins shell: %w", err)
		}
	}
	return nil
}

// migratePlansToTeams renames the top-level "plans" concept to "teams".
// It is idempotent: fresh databases already use the new names and are left untouched.
func migratePlansToTeams(db *DB) error {
	if tableExists(db, "plans") && !tableExists(db, "teams") {
		log.Println("⚡ Migrating: renaming plans table to teams")
		if _, err := db.Exec("ALTER TABLE plans RENAME TO teams"); err != nil {
			return fmt.Errorf("rename plans to teams: %w", err)
		}
	}
	if tableExists(db, "plan_admins") && !tableExists(db, "team_admins") {
		log.Println("⚡ Migrating: renaming plan_admins table to team_admins")
		if _, err := db.Exec("ALTER TABLE plan_admins RENAME TO team_admins"); err != nil {
			return fmt.Errorf("rename plan_admins to team_admins: %w", err)
		}
	}
	// Fallback: both generations exist with data (should not happen via normal
	// upgrades). Merge legacy rows, then drop the legacy tables.
	if tableExists(db, "plans") && tableExists(db, "teams") {
		log.Println("⚠️ Migrating: both plans and teams hold data; merging into teams")
		if columnExists(db, "plans", "jira_url") {
			_, err := db.Exec(`
				INSERT INTO teams (id, name, created_at, updated_at, jira_url, jira_user, jira_token, jira_jql, jira_sp_field, jira_insecure)
				SELECT id, name, created_at, updated_at, jira_url, jira_user, jira_token, jira_jql, jira_sp_field, jira_insecure
				FROM plans WHERE id NOT IN (SELECT id FROM teams)`)
			if err != nil {
				return fmt.Errorf("merge plans into teams: %w", err)
			}
		} else {
			_, err := db.Exec(`INSERT INTO teams (id, name) SELECT id, name FROM plans WHERE id NOT IN (SELECT id FROM teams)`)
			if err != nil {
				return fmt.Errorf("merge plans into teams: %w", err)
			}
		}
		if _, err := db.Exec("DROP TABLE plans"); err != nil {
			return fmt.Errorf("drop merged plans table: %w", err)
		}
	}
	if tableExists(db, "plan_admins") && tableExists(db, "team_admins") {
		log.Println("⚠️ Migrating: both plan_admins and team_admins hold data; merging")
		if _, err := db.Exec(`INSERT OR IGNORE INTO team_admins (team_id, user_id) SELECT plan_id, user_id FROM plan_admins`); err != nil {
			return fmt.Errorf("merge plan_admins into team_admins: %w", err)
		}
		if _, err := db.Exec("DROP TABLE plan_admins"); err != nil {
			return fmt.Errorf("drop merged plan_admins table: %w", err)
		}
	}

	for _, table := range []string{"capacity_plans", "presentations", "jira_snapshots", "team_admins"} {
		if !tableExists(db, table) {
			continue
		}
		if columnExists(db, table, "plan_id") && !columnExists(db, table, "team_id") {
			log.Printf("⚡ Migrating: renaming %s.plan_id to team_id\n", table)
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s RENAME COLUMN plan_id TO team_id", table)); err != nil {
				return fmt.Errorf("rename %s.plan_id: %w", table, err)
			}
		}
	}

	// Replace legacy plan_id indexes with team_id equivalents.
	for _, stmt := range []string{
		"DROP INDEX IF EXISTS idx_capacity_plans_plan_id",
		"DROP INDEX IF EXISTS idx_presentations_plan_id",
		"DROP INDEX IF EXISTS idx_jira_snapshots_plan_id",
		"CREATE INDEX IF NOT EXISTS idx_capacity_plans_team_id ON capacity_plans(team_id)",
		"CREATE INDEX IF NOT EXISTS idx_presentations_team_id ON presentations(team_id)",
		"CREATE INDEX IF NOT EXISTS idx_jira_snapshots_team_id ON jira_snapshots(team_id)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("update team indexes (%s): %w", stmt, err)
		}
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

func superAdminUsername() string {
	for _, key := range []string{"SCRUMY_SUPERADMIN_USERNAME", "SCRUMY_SUPERADMIN", "SUPER_ADMIN_USERNAME"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return "root"
}

func superAdminInitialPassword() string {
	if v := os.Getenv("SCRUMY_SUPERADMIN_PASSWORD"); v != "" {
		return v
	}
	return "root"
}

func (db *DB) SeedRootUser() error {
	superAdmin := superAdminUsername()
	superPassword := superAdminInitialPassword()

	// Ensure the configured super admin exists with the admin role.
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", superAdmin).Scan(&count)
	if err != nil {
		return err
	}
	var superAdminID string
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(superPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		superAdminID = uuid.New().String()
		_, err = db.Exec("INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)",
			superAdminID, superAdmin, string(hash), "admin")
		if err != nil {
			return err
		}
		log.Printf("🌱 Seeded super admin user %q\n", superAdmin)
	} else {
		// The super admin username already exists: skip creation entirely so
		// the existing password hash is left untouched, and treat the
		// existing user as the superuser by ensuring it holds the admin role.
		if err := db.QueryRow("SELECT id FROM users WHERE username = ?", superAdmin).Scan(&superAdminID); err != nil {
			return err
		}
		if _, err := db.Exec("UPDATE users SET role = 'admin' WHERE username = ? AND role != 'admin'", superAdmin); err != nil {
			return err
		}
	}

	// Link all existing teams to the super admin if they don't have an admin
	rows, err := db.Query("SELECT id FROM teams WHERE id NOT IN (SELECT team_id FROM team_admins)")
	if err != nil {
		return err
	}
	defer rows.Close()

	var teamIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		teamIDs = append(teamIDs, id)
	}

	for _, teamID := range teamIDs {
		_, err = db.Exec("INSERT INTO team_admins (team_id, user_id) VALUES (?, ?)", teamID, superAdminID)
		if err != nil {
			return err
		}
	}
	return nil
}

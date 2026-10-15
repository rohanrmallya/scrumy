package db

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// If the configured super admin username already exists in the DB, startup
// must skip creation (leaving the existing password untouched) and treat the
// existing user as the superuser (admin role).
func TestSeedSkipsExistingSuperuser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	// Open once with defaults to create a plain database.
	database, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Simulate a pre-existing deployment user: username "boss" with role
	// "user" and a known password.
	const legacyPassword = "legacy-pass-123"
	hash, err := bcrypt.GenerateFromPassword([]byte(legacyPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	bossID := uuid.New().String()
	if _, err := database.Exec(
		"INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)",
		bossID, "boss", string(hash), "user"); err != nil {
		t.Fatalf("insert pre-existing user: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Reopen with "boss" configured as the super admin and a *different*
	// seed password: creation must be skipped, so the seed password must NOT
	// take effect.
	t.Setenv("SCRUMY_SUPERADMIN_USERNAME", "boss")
	t.Setenv("SCRUMY_SUPERADMIN_PASSWORD", "brand-new-seed-pass")

	database, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer database.Close()

	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'boss'").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 'boss' row after reseed, got %d", count)
	}

	var id, storedHash, role string
	if err := database.QueryRow("SELECT id, password_hash, role FROM users WHERE username = 'boss'").
		Scan(&id, &storedHash, &role); err != nil {
		t.Fatalf("select: %v", err)
	}
	if id != bossID {
		t.Fatalf("superuser row was replaced: id changed from %s to %s", bossID, id)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(legacyPassword)); err != nil {
		t.Fatal("existing password hash was overwritten by the seed password")
	}
	if role != "admin" {
		t.Fatalf("existing superuser was not treated as superuser: role = %q, want %q", role, "admin")
	}

	// A further restart must remain a no-op (still exactly one row).
	if err := database.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	database, err = Open(path)
	if err != nil {
		t.Fatalf("second reopen: %v", err)
	}
	defer database.Close()
	if err := database.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'boss'").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 'boss' row after second reseed, got %d", count)
	}
}

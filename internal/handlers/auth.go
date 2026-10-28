package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"scrumy/internal/db"
	"scrumy/internal/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	DB *db.DB
}

type contextKey string

const userContextKey contextKey = "user"

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request")
		return
	}

	var user models.User
	var passwordHash string
	err := h.DB.QueryRow("SELECT id, username, password_hash, role FROM users WHERE username = ?", req.Username).
		Scan(&user.ID, &user.Username, &passwordHash, &user.Role)
	if err == sql.ErrNoRows {
		respondErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	} else if err != nil {
		respondErr(w, http.StatusInternalServerError, "db error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		respondErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	sessionID := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)
	_, err = h.DB.Exec("INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)", sessionID, user.ID, expiresAt)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	respond(w, http.StatusOK, user)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondErr(w, http.StatusBadRequest, "username and password required")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	userID := uuid.New().String()
	_, err = h.DB.Exec("INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)",
		userID, req.Username, string(hash), "user")
	if err != nil {
		respondErr(w, http.StatusConflict, "username already exists")
		return
	}

	respond(w, http.StatusCreated, map[string]string{"id": userID})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		h.DB.Exec("DELETE FROM sessions WHERE id = ?", cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	respond(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := h.GetUserFromContext(r.Context())
	if user == nil {
		respondErr(w, http.StatusUnauthorized, "not logged in")
		return
	}
	respond(w, http.StatusOK, user)
}

func (h *AuthHandler) GetUser(r *http.Request) *models.User {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil
	}

	var user models.User
	err = h.DB.QueryRow(`
		SELECT u.id, u.username, u.role 
		FROM users u 
		JOIN sessions s ON u.id = s.user_id 
		WHERE s.id = ? AND s.expires_at > CURRENT_TIMESTAMP`,
		cookie.Value).Scan(&user.ID, &user.Username, &user.Role)
	if err != nil {
		return nil
	}

	return &user
}

func (h *AuthHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := h.GetUser(r)
		if user != nil {
			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func (h *AuthHandler) GetUserFromContext(ctx context.Context) *models.User {
	user, ok := ctx.Value(userContextKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}

func (h *AuthHandler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := h.GetUserFromContext(r.Context())
		if user == nil {
			respondErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *AuthHandler) IsTeamAdmin(userID, teamID string) bool {
	if userID != "" {
		var username, role string
		if err := h.DB.QueryRow("SELECT username, role FROM users WHERE id = ?", userID).Scan(&username, &role); err == nil {
			if role == "admin" || username == SuperAdminUsername() {
				return true
			}
		}
	}
	var count int
	err := h.DB.QueryRow("SELECT COUNT(*) FROM team_admins WHERE team_id = ? AND user_id = ?", teamID, userID).Scan(&count)
	return err == nil && count > 0
}

// IsPlanAdmin is a deprecated alias for IsTeamAdmin, kept for backward compatibility.
func (h *AuthHandler) IsPlanAdmin(userID, planID string) bool {
	return h.IsTeamAdmin(userID, planID)
}

// ─── Super Admin ─────────────────────────────────────────────────────────────
// The super admin username is configurable via env (default: "root").
// Resolution order: SCRUMY_SUPERADMIN_USERNAME > SCRUMY_SUPERADMIN > SUPER_ADMIN_USERNAME.

func SuperAdminUsername() string {
	for _, key := range []string{"SCRUMY_SUPERADMIN_USERNAME", "SCRUMY_SUPERADMIN", "SUPER_ADMIN_USERNAME"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return "root"
}

func SuperAdminInitialPassword() string {
	if v := os.Getenv("SCRUMY_SUPERADMIN_PASSWORD"); v != "" {
		return v
	}
	return "root"
}

// IsSuperAdmin reports whether the user is the super admin, either by matching
// the configured super admin username or by holding the global admin role.
func IsSuperAdmin(user *models.User) bool {
	if user == nil {
		return false
	}
	if user.Role == "admin" {
		return true
	}
	return user.Username == SuperAdminUsername()
}

func (h *AuthHandler) RequireSuperAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := h.GetUserFromContext(r.Context())
		if user == nil {
			respondErr(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if !IsSuperAdmin(user) {
			respondErr(w, http.StatusForbidden, "super admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ChangePassword lets any logged-in user change their own password.
// Requires the current password to guard against session hijacking.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := h.GetUserFromContext(r.Context())
	if user == nil {
		respondErr(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := decode(r, &req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		respondErr(w, http.StatusBadRequest, "current_password and new_password are required")
		return
	}
	if len(req.NewPassword) < 4 {
		respondErr(w, http.StatusBadRequest, "new password must be at least 4 characters")
		return
	}
	if req.CurrentPassword == req.NewPassword {
		respondErr(w, http.StatusBadRequest, "new password must differ from current password")
		return
	}

	var passwordHash string
	err := h.DB.QueryRow("SELECT password_hash FROM users WHERE id = ?", user.ID).Scan(&passwordHash)
	if err == sql.ErrNoRows {
		respondErr(w, http.StatusUnauthorized, "user not found")
		return
	} else if err != nil {
		respondErr(w, http.StatusInternalServerError, "db error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.CurrentPassword)); err != nil {
		respondErr(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if _, err := h.DB.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(newHash), user.ID); err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	// Invalidate all other sessions for this user, keeping the current one alive.
	if cookie, err := r.Cookie("session_id"); err == nil && cookie.Value != "" {
		h.DB.Exec("DELETE FROM sessions WHERE user_id = ? AND id != ?", user.ID, cookie.Value)
	} else {
		h.DB.Exec("DELETE FROM sessions WHERE user_id = ?", user.ID)
	}

	respond(w, http.StatusOK, map[string]string{"message": "password changed"})
}

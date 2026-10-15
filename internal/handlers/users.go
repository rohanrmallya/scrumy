package handlers

import (
	"database/sql"
	"net/http"
	"scrumy/internal/db"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func trimSearchQuery(s string) string {
	return strings.TrimSpace(s)
}

// UsersHandler manages global user accounts. Most endpoints require super admin;
// Search is available to any authenticated user for admin pickers.
type UsersHandler struct {
	DB   *db.DB
	Auth *AuthHandler
}

type userResponse struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	CreatedAt   string `json:"created_at"`
	IsSuperAdmin bool  `json:"is_super_admin,omitempty"`
}

func scanUserRow(id, username, role, createdAt string) userResponse {
	// Normalize created_at to RFC3339 when possible; fall back to raw value.
	formatted := createdAt
	for _, layout := range []string{"2006-01-02T15:04:05Z", "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, createdAt); err == nil {
			formatted = t.UTC().Format(time.RFC3339)
			break
		}
	}
	return userResponse{
		ID:           id,
		Username:     username,
		Role:         role,
		CreatedAt:    formatted,
		IsSuperAdmin: username == SuperAdminUsername(),
	}
}

func (h *UsersHandler) requireSuperAdmin(w http.ResponseWriter, r *http.Request) bool {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil {
		respondErr(w, http.StatusUnauthorized, "authentication required")
		return false
	}
	if !IsSuperAdmin(user) {
		respondErr(w, http.StatusForbidden, "super admin required")
		return false
	}
	return true
}

// Search returns matching users (id + username only) for admin pickers.
// Any authenticated user may call it; results are capped to avoid dumping.
func (h *UsersHandler) Search(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil {
		respondErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	q := r.URL.Query().Get("q")
	// Support both ?q= and ?query= / ?search= for convenience.
	if q == "" {
		q = r.URL.Query().Get("query")
	}
	if q == "" {
		q = r.URL.Query().Get("search")
	}
	q = trimSearchQuery(q)
	if q == "" {
		respond(w, http.StatusOK, []map[string]string{})
		return
	}
	rows, err := h.DB.Query(
		"SELECT id, username FROM users WHERE username LIKE ? ORDER BY username ASC LIMIT 20",
		"%"+q+"%",
	)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	results := []map[string]string{}
	for rows.Next() {
		var id, username string
		if err := rows.Scan(&id, &username); err != nil {
			continue
		}
		results = append(results, map[string]string{"id": id, "username": username})
	}
	respond(w, http.StatusOK, results)
}

// List returns all users (super admin only).
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {	if !h.requireSuperAdmin(w, r) {
		return
	}
	rows, err := h.DB.Query("SELECT id, username, role, created_at FROM users ORDER BY username ASC")
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "db error")
		return
	}
	defer rows.Close()

	users := []userResponse{}
	for rows.Next() {
		var id, username, role, createdAt string
		if err := rows.Scan(&id, &username, &role, &createdAt); err != nil {
			respondErr(w, http.StatusInternalServerError, "db error")
			return
		}
		users = append(users, scanUserRow(id, username, role, createdAt))
	}
	respond(w, http.StatusOK, users)
}

// Create adds a new user (super admin only).
func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.requireSuperAdmin(w, r) {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decode(r, &req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Username == "" || req.Password == "" {
		respondErr(w, http.StatusBadRequest, "username and password required")
		return
	}
	if len(req.Password) < 4 {
		respondErr(w, http.StatusBadRequest, "password must be at least 4 characters")
		return
	}
	role := req.Role
	if role == "" {
		role = "user"
	}
	if role != "admin" && role != "user" {
		respondErr(w, http.StatusBadRequest, "role must be admin or user")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	userID := uuid.New().String()
	_, err = h.DB.Exec("INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)",
		userID, req.Username, string(hash), role)
	if err != nil {
		respondErr(w, http.StatusConflict, "username already exists")
		return
	}

	var createdAt string
	_ = h.DB.QueryRow("SELECT created_at FROM users WHERE id = ?", userID).Scan(&createdAt)
	respond(w, http.StatusCreated, scanUserRow(userID, req.Username, role, createdAt))
}

// UpdateRole changes a user's global role (super admin only).
func (h *UsersHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	if !h.requireSuperAdmin(w, r) {
		return
	}
	requester := h.Auth.GetUserFromContext(r.Context())
	targetID := chi.URLParam(r, "userID")

	var req struct {
		Role string `json:"role"`
	}
	if err := decode(r, &req); err != nil || (req.Role != "admin" && req.Role != "user") {
		respondErr(w, http.StatusBadRequest, "role must be admin or user")
		return
	}

	var targetUsername, targetRole string
	err := h.DB.QueryRow("SELECT username, role FROM users WHERE id = ?", targetID).Scan(&targetUsername, &targetRole)
	if err == sql.ErrNoRows {
		respondErr(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		respondErr(w, http.StatusInternalServerError, "db error")
		return
	}

	if targetUsername == SuperAdminUsername() {
		respondErr(w, http.StatusBadRequest, "cannot change the super admin account role")
		return
	}
	if requester != nil && targetID == requester.ID {
		respondErr(w, http.StatusBadRequest, "cannot change your own role")
		return
	}

	if _, err := h.DB.Exec("UPDATE users SET role = ? WHERE id = ?", req.Role, targetID); err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	var createdAt string
	_ = h.DB.QueryRow("SELECT created_at FROM users WHERE id = ?", targetID).Scan(&createdAt)
	respond(w, http.StatusOK, scanUserRow(targetID, targetUsername, req.Role, createdAt))
}

// ResetPassword sets a new password for any user without knowing the old one (super admin only).
func (h *UsersHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	if !h.requireSuperAdmin(w, r) {
		return
	}
	requester := h.Auth.GetUserFromContext(r.Context())
	targetID := chi.URLParam(r, "userID")

	var req struct {
		Password    string `json:"password"`
		NewPassword string `json:"new_password"`
	}
	if err := decode(r, &req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request")
		return
	}
	newPassword := req.NewPassword
	if newPassword == "" {
		newPassword = req.Password
	}
	if newPassword == "" {
		respondErr(w, http.StatusBadRequest, "new_password is required")
		return
	}
	if len(newPassword) < 4 {
		respondErr(w, http.StatusBadRequest, "password must be at least 4 characters")
		return
	}

	var exists int
	if err := h.DB.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", targetID).Scan(&exists); err != nil || exists == 0 {
		respondErr(w, http.StatusNotFound, "user not found")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if _, err := h.DB.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hash), targetID); err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	// Force re-login for the target user. Keep the requester's own session alive
	// when the super admin resets their own password via this endpoint.
	if requester != nil && targetID == requester.ID {
		if cookie, err := r.Cookie("session_id"); err == nil && cookie.Value != "" {
			h.DB.Exec("DELETE FROM sessions WHERE user_id = ? AND id != ?", targetID, cookie.Value)
		} else {
			h.DB.Exec("DELETE FROM sessions WHERE user_id = ?", targetID)
		}
	} else {
		h.DB.Exec("DELETE FROM sessions WHERE user_id = ?", targetID)
	}

	respond(w, http.StatusOK, map[string]string{"message": "password updated"})
}

// Delete removes a user (super admin only).
func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requireSuperAdmin(w, r) {
		return
	}
	requester := h.Auth.GetUserFromContext(r.Context())
	targetID := chi.URLParam(r, "userID")

	var targetUsername string
	err := h.DB.QueryRow("SELECT username FROM users WHERE id = ?", targetID).Scan(&targetUsername)
	if err == sql.ErrNoRows {
		respondErr(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		respondErr(w, http.StatusInternalServerError, "db error")
		return
	}

	if requester != nil && targetID == requester.ID {
		respondErr(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}
	if targetUsername == SuperAdminUsername() {
		respondErr(w, http.StatusBadRequest, "cannot delete the super admin account")
		return
	}

	// Explicitly clear dependent rows for DBs created before FK enforcement,
	// then delete the user (FK cascade covers newer DBs).
	h.DB.Exec("DELETE FROM sessions WHERE user_id = ?", targetID)
	h.DB.Exec("DELETE FROM plan_admins WHERE user_id = ?", targetID)
	res, err := h.DB.Exec("DELETE FROM users WHERE id = ?", targetID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, "failed to delete user")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		respondErr(w, http.StatusNotFound, "user not found")
		return
	}
	respond(w, http.StatusOK, map[string]bool{"deleted": true})
}

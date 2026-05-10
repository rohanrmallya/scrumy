package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"scrumy/internal/db"
	"scrumy/internal/models"
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

func (h *AuthHandler) IsPlanAdmin(userID, planID string) bool {
	var count int
	err := h.DB.QueryRow("SELECT COUNT(*) FROM plan_admins WHERE plan_id = ? AND user_id = ?", planID, userID).Scan(&count)
	return err == nil && count > 0
}

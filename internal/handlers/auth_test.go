package handlers

import (
	"testing"

	"scrumy/internal/models"
)

func TestIsSuperAdmin(t *testing.T) {
	t.Setenv("SCRUMY_SUPERADMIN_USERNAME", "boss")

	cases := []struct {
		name string
		user *models.User
		want bool
	}{
		{"nil user", nil, false},
		{"existing superuser by username without admin role", &models.User{ID: "1", Username: "boss", Role: "user"}, true},
		{"superuser with admin role", &models.User{ID: "1", Username: "boss", Role: "admin"}, true},
		{"other admin by role", &models.User{ID: "2", Username: "alice", Role: "admin"}, true},
		{"regular user", &models.User{ID: "3", Username: "alice", Role: "user"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSuperAdmin(tc.user); got != tc.want {
				t.Fatalf("IsSuperAdmin() = %v, want %v", got, tc.want)
			}
		})
	}
}

package main

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"scrumy/internal/db"
	"scrumy/internal/handlers"
	"scrumy/internal/scheduler"
	"strings"
	"time"

	"scrumy"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	log.Println("Starting Scrumy server...")
	dbPath := os.Getenv("SCRUMY_DB")
	if dbPath == "" {
		dbPath = "scrumy.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:8080"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
	}))

	// ── API routes ───────────────────────────────────────────────────────────
	authH := &handlers.AuthHandler{DB: database}
	usersH := &handlers.UsersHandler{DB: database, Auth: authH}
	teamsH := &handlers.TeamsHandler{DB: database, Auth: authH}
	capacityH := &handlers.CapacityHandler{DB: database, Auth: authH}
	presH := &handlers.PresentationsHandler{DB: database, Auth: authH}
	jiraH := &handlers.JiraHandler{DB: database, Auth: authH}

	// ── Auto-Refresh Background Scheduler ─────────────────────────────────────
	sched := scheduler.New(database, func(teamID, snapshotID, triggerType string) error {
		_, err := jiraH.ExecuteAutoRefresh(teamID, snapshotID, triggerType)
		return err
	}, 30*time.Second)
	sched.Start()
	defer sched.Stop()

	registerTeamRoutes := func(r chi.Router, prefix string) {
		// Teams
		r.Get(prefix, teamsH.List)
		r.Post(prefix, teamsH.Create)
		r.Get(prefix+"/{teamID}", teamsH.Get)
		r.Put(prefix+"/{teamID}", teamsH.Update)
		r.Delete(prefix+"/{teamID}", teamsH.Delete)
		r.Post(prefix+"/{teamID}/admins", teamsH.AddAdmin)
		r.Delete(prefix+"/{teamID}/admins", teamsH.RemoveAdmin)

		// Jira Integration
		r.Put(prefix+"/{teamID}/jira/settings", jiraH.UpdateSettings)
		r.Post(prefix+"/{teamID}/jira/test-connection", jiraH.TestConnection)
		r.Post(prefix+"/{teamID}/jira/snapshots", jiraH.CreateSnapshot)
		r.Get(prefix+"/{teamID}/jira/snapshots", jiraH.ListSnapshots)
		r.Get(prefix+"/{teamID}/jira/snapshots/{snapshotID}", jiraH.GetSnapshot)
		r.Put(prefix+"/{teamID}/jira/snapshots/{snapshotID}", jiraH.UpdateSnapshot)
		r.Post(prefix+"/{teamID}/jira/snapshots/{snapshotID}/refresh", jiraH.RefreshSnapshot)
		r.Delete(prefix+"/{teamID}/jira/snapshots/{snapshotID}", jiraH.DeleteSnapshot)
		r.Post(prefix+"/{teamID}/jira/snapshots/{snapshotID}/issues/{issueKey}/toggle-planned", jiraH.ToggleIssuePlanned)

		// Jira Auto-Refresh Jobs & Audits
		r.Get(prefix+"/{teamID}/jira/auto-refresh/jobs", jiraH.GetTeamAutoRefreshJobs)
		r.Get(prefix+"/{teamID}/jira/auto-refresh/audits", jiraH.GetTeamAutoRefreshAudits)
		r.Put(prefix+"/{teamID}/jira/snapshots/{snapshotID}/auto-refresh", jiraH.UpdateAutoRefreshSettings)
		r.Post(prefix+"/{teamID}/jira/snapshots/{snapshotID}/auto-refresh/disable", jiraH.DisableAutoRefresh)
		r.Post(prefix+"/{teamID}/jira/snapshots/{snapshotID}/auto-refresh/run", jiraH.RunAutoRefreshNow)

		// Capacity Plans
		r.Get(prefix+"/{teamID}/capacity", capacityH.List)
		r.Post(prefix+"/{teamID}/capacity", capacityH.Create)
		r.Get(prefix+"/{teamID}/capacity/{cpID}", capacityH.Get)
		r.Put(prefix+"/{teamID}/capacity/{cpID}", capacityH.Update)
		r.Delete(prefix+"/{teamID}/capacity/{cpID}", capacityH.Delete)
		r.Get(prefix+"/{teamID}/capacity/{cpID}/summary", capacityH.Summary)

		// Members
		r.Post(prefix+"/{teamID}/capacity/{cpID}/members", capacityH.AddMember)
		r.Put(prefix+"/{teamID}/capacity/{cpID}/members/{memberID}", capacityH.UpdateMember)
		r.Delete(prefix+"/{teamID}/capacity/{cpID}/members/{memberID}", capacityH.DeleteMember)

		// Sprints
		r.Post(prefix+"/{teamID}/capacity/{cpID}/sprints", capacityH.AddSprint)
		r.Put(prefix+"/{teamID}/capacity/{cpID}/sprints/{sprintID}", capacityH.UpdateSprint)
		r.Delete(prefix+"/{teamID}/capacity/{cpID}/sprints/{sprintID}", capacityH.DeleteSprint)
		r.Post(prefix+"/{teamID}/capacity/{cpID}/sprints/{sprintID}/leaves", capacityH.UpsertLeave)

		// Presentations
		r.Get(prefix+"/{teamID}/presentations", presH.List)
		r.Post(prefix+"/{teamID}/presentations", presH.Create)
		r.Get(prefix+"/{teamID}/presentations/{presID}", presH.Get)
		r.Put(prefix+"/{teamID}/presentations/{presID}", presH.Update)
		r.Delete(prefix+"/{teamID}/presentations/{presID}", presH.Delete)
		r.Post(prefix+"/{teamID}/presentations/{presID}/publish", presH.Publish)
		r.Post(prefix+"/{teamID}/presentations/{presID}/unpublish", presH.Unpublish)
		r.Post(prefix+"/{teamID}/presentations/{presID}/feedback", presH.AddRetroFeedback)
	}

	r.Route("/api", func(r chi.Router) {
		r.Use(authH.Middleware)

		// Auth
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/logout", authH.Logout)
		r.Get("/auth/me", authH.Me)
		r.Post("/auth/change-password", authH.ChangePassword)

		// Users (super admin only, except search)
		r.Get("/users/search", usersH.Search)
		r.Get("/users", usersH.List)
		r.Post("/users", usersH.Create)
		r.Put("/users/{userID}", usersH.UpdateRole)
		r.Put("/users/{userID}/password", usersH.ResetPassword)
		r.Delete("/users/{userID}", usersH.Delete)

		// Super admin operations
		r.Group(func(r chi.Router) {
			r.Use(authH.RequireSuperAdmin)
			r.Post("/admin/snapshots/refresh-all", jiraH.RefreshAllSnapshots)
			r.Post("/admin/teams/add-admin", teamsH.AddAdminToAllTeams)
			r.Get("/admin/auto-refresh/jobs", jiraH.GetAllAutoRefreshJobs)
			r.Get("/admin/auto-refresh/audits", jiraH.GetAllAutoRefreshAudits)
			r.Post("/admin/snapshots/{snapshotID}/auto-refresh/disable", jiraH.SuperAdminDisableAutoRefresh)
			r.Post("/admin/snapshots/{snapshotID}/auto-refresh/run", jiraH.SuperAdminRunAutoRefresh)
		})

		// Teams (current) + deprecated /plans aliases for backward compatibility
		registerTeamRoutes(r, "/teams")
		registerTeamRoutes(r, "/plans")
	})

	// ── Static frontend ──────────────────────────────────────────────────────
	distFS, err := fs.Sub(scrumy.WebFS, "web/dist")
	if err != nil {
		log.Fatalf("could not sub web/dist: %v", err)
	}

	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		urlPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		// Deprecated /plans/* page URLs redirect to their /teams/* equivalents.
		// (/plans alone has no counterpart, so it goes home.)
		if urlPath == "plans" || strings.HasPrefix(urlPath, "plans/") {
			rest := strings.TrimPrefix(urlPath, "plans")
			rest = strings.TrimPrefix(rest, "/")
			target := "/"
			if rest != "" {
				target = "/teams/" + rest
			}
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
		if urlPath == "" || urlPath == "." {
			urlPath = "index.html"
		}

		// Try to open the file
		f, err := distFS.Open(urlPath)
		if err != nil {
			// Fallback to index.html for SPA routing
			urlPath = "index.html"
			f, err = distFS.Open(urlPath)
			if err != nil {
				http.Error(w, "index.html not found", http.StatusInternalServerError)
				return
			}
		}
		defer f.Close()

		stat, _ := f.Stat()
		if stat.IsDir() {
			// If it's a directory, serve index.html from inside it
			urlPath = path.Join(urlPath, "index.html")
			f2, err := distFS.Open(urlPath)
			if err != nil {
				http.Error(w, "index.html not found", http.StatusInternalServerError)
				return
			}
			defer f2.Close()
			f = f2
			stat, _ = f.Stat()
		}

		http.ServeContent(w, r, urlPath, stat.ModTime(), f.(io.ReadSeeker))
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🚀 Scrumy running at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

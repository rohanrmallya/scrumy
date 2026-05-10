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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"scrumy"
)

func main() {
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
	plansH := &handlers.PlansHandler{DB: database, Auth: authH}
	capacityH := &handlers.CapacityHandler{DB: database, Auth: authH}
	presH := &handlers.PresentationsHandler{DB: database, Auth: authH}

	r.Route("/api", func(r chi.Router) {
		r.Use(authH.Middleware)

		// Auth
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/logout", authH.Logout)
		r.Get("/auth/me", authH.Me)

		// Plans
		r.Get("/plans", plansH.List)
		r.Post("/plans", plansH.Create)
		r.Get("/plans/{planID}", plansH.Get)
		r.Put("/plans/{planID}", plansH.Update)
		r.Delete("/plans/{planID}", plansH.Delete)
		r.Post("/plans/{planID}/admins", plansH.AddAdmin)
		r.Delete("/plans/{planID}/admins", plansH.RemoveAdmin)

		// Capacity Plans
		r.Get("/plans/{planID}/capacity", capacityH.List)
		r.Post("/plans/{planID}/capacity", capacityH.Create)
		r.Get("/plans/{planID}/capacity/{cpID}", capacityH.Get)
		r.Put("/plans/{planID}/capacity/{cpID}", capacityH.Update)
		r.Delete("/plans/{planID}/capacity/{cpID}", capacityH.Delete)
		r.Get("/plans/{planID}/capacity/{cpID}/summary", capacityH.Summary)

		// Members
		r.Post("/plans/{planID}/capacity/{cpID}/members", capacityH.AddMember)
		r.Put("/plans/{planID}/capacity/{cpID}/members/{memberID}", capacityH.UpdateMember)
		r.Delete("/plans/{planID}/capacity/{cpID}/members/{memberID}", capacityH.DeleteMember)

		// Sprints
		r.Post("/plans/{planID}/capacity/{cpID}/sprints", capacityH.AddSprint)
		r.Put("/plans/{planID}/capacity/{cpID}/sprints/{sprintID}", capacityH.UpdateSprint)
		r.Delete("/plans/{planID}/capacity/{cpID}/sprints/{sprintID}", capacityH.DeleteSprint)
		r.Post("/plans/{planID}/capacity/{cpID}/sprints/{sprintID}/leaves", capacityH.UpsertLeave)

		// Presentations
		r.Get("/plans/{planID}/presentations", presH.List)
		r.Post("/plans/{planID}/presentations", presH.Create)
		r.Get("/plans/{planID}/presentations/{presID}", presH.Get)
		r.Put("/plans/{planID}/presentations/{presID}", presH.Update)
		r.Delete("/plans/{planID}/presentations/{presID}", presH.Delete)
		r.Post("/plans/{planID}/presentations/{presID}/publish", presH.Publish)
		r.Post("/plans/{planID}/presentations/{presID}/unpublish", presH.Unpublish)
		r.Post("/plans/{planID}/presentations/{presID}/feedback", presH.AddRetroFeedback)
	})

	// ── Static frontend ──────────────────────────────────────────────────────
	distFS, err := fs.Sub(scrumy.WebFS, "web/dist")
	if err != nil {
		log.Fatalf("could not sub web/dist: %v", err)
	}

	staticServer := http.FileServer(http.FS(distFS))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path
		// Check if file exists in the embedded FS
		f, err := distFS.Open(path.Clean(urlPath))
		if err == nil {
			f.Close()
			staticServer.ServeHTTP(w, r)
			return
		}
		
		// Fallback to index.html for SPA routing
		index, err := distFS.Open("index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		defer index.Close()
		stat, _ := index.Stat()
		http.ServeContent(w, r, "index.html", stat.ModTime(), index.(io.ReadSeeker))
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🚀 Scrumy running at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

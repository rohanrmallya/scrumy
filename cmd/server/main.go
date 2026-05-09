package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
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
	plansH := &handlers.PlansHandler{DB: database}
	capacityH := &handlers.CapacityHandler{DB: database}
	presH := &handlers.PresentationsHandler{DB: database}

	r.Route("/api", func(r chi.Router) {
		// Plans
		r.Get("/plans", plansH.List)
		r.Post("/plans", plansH.Create)
		r.Get("/plans/{planID}", plansH.Get)
		r.Put("/plans/{planID}", plansH.Update)
		r.Delete("/plans/{planID}", plansH.Delete)

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
	r.Get("/*", http.FileServer(http.FS(distFS)).ServeHTTP)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🚀 Scrumy running at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"notes-api/cache"
	"notes-api/config"
	"notes-api/database"
	"notes-api/handlers"
	"notes-api/middleware"
	"notes-api/repository"
	"notes-api/service"
)

func main() {
	cfg := config.LoadConfig()

	log.Printf("Starting notes-api (native net/http) on port %s...", cfg.Port)

	// Connect to Database
	db, err := database.ConnectPostgres(cfg)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	// Connect to Redis
	redisCache, err := cache.ConnectRedis(cfg)
	if err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}

	// Initialize Repositories
	userRepo := repository.NewUserRepository(db)
	noteRepo := repository.NewNoteRepository(db)

	// Initialize Services
	authService := service.NewAuthService(userRepo, redisCache, cfg)
	noteService := service.NewNoteService(noteRepo, userRepo)

	// Initialize Handlers
	authHandler := handlers.NewAuthHandler(authService)
	noteHandler := handlers.NewNoteHandler(noteService)

	// Setup Router (Go 1.22+ ServeMux with method & path routing)
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "ok"
		if err := db.Ping(); err != nil {
			dbStatus = err.Error()
		}
		redisStatus := "ok"
		if err := redisCache.Ping(r.Context()); err != nil {
			redisStatus = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
			"app":    "notes-api",
			"driver": "net/http (native)",
			"db":     dbStatus,
			"redis":  redisStatus,
		})
	})

	// Public Auth routes
	mux.HandleFunc("POST /api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", authHandler.RefreshToken)

	// Protected routes wrapper
	withAuth := middleware.Auth(authService)

	mux.Handle("POST /api/v1/auth/logout", withAuth(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("GET /api/v1/auth/me", withAuth(http.HandlerFunc(authHandler.Me)))

	// Notes CRUD & Sharing
	mux.Handle("POST /api/v1/notes", withAuth(http.HandlerFunc(noteHandler.CreateNote)))
	mux.Handle("GET /api/v1/notes", withAuth(http.HandlerFunc(noteHandler.ListNotes)))
	mux.Handle("GET /api/v1/notes/{id}", withAuth(http.HandlerFunc(noteHandler.GetNote)))
	mux.Handle("PUT /api/v1/notes/{id}", withAuth(http.HandlerFunc(noteHandler.UpdateNote)))
	mux.Handle("DELETE /api/v1/notes/{id}", withAuth(http.HandlerFunc(noteHandler.DeleteNote)))

	// Share management & authorization check
	mux.Handle("POST /api/v1/notes/{id}/share", withAuth(http.HandlerFunc(noteHandler.ShareNote)))
	mux.Handle("GET /api/v1/notes/{id}/shares", withAuth(http.HandlerFunc(noteHandler.GetNoteShares)))
	mux.Handle("DELETE /api/v1/notes/{id}/share/{userId}", withAuth(http.HandlerFunc(noteHandler.UnshareNote)))
	mux.Handle("GET /api/v1/notes/{id}/access", withAuth(http.HandlerFunc(noteHandler.CheckAccess)))

	// Global Middlewares: Logger and CORS
	handler := middleware.Logger(middleware.CORS(mux))

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server ListenAndServe error: %v", err)
		}
	}()

	log.Printf("notes-api successfully running on :%s", cfg.Port)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down notes-api...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("notes-api exited cleanly")
}


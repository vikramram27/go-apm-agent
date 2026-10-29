package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"notes-api-gin/cache"
	"notes-api-gin/config"
	"notes-api-gin/database"
	"notes-api-gin/handlers"
	"notes-api-gin/middleware"
	"notes-api-gin/repository"
	"notes-api-gin/service"
)

func main() {
	cfg := config.LoadConfig()

	log.Printf("Starting notes-api-gin (Gin framework) on port %s...", cfg.Port)

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

	// Setup Gin Engine
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(middleware.CORS())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		dbStatus := "ok"
		if err := db.Ping(); err != nil {
			dbStatus = err.Error()
		}
		redisStatus := "ok"
		if err := redisCache.Ping(c.Request.Context()); err != nil {
			redisStatus = err.Error()
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"app":    "notes-api-gin",
			"driver": "gin-gonic/gin",
			"db":     dbStatus,
			"redis":  redisStatus,
		})
	})

	// API v1 Group
	v1 := router.Group("/api/v1")
	{
		// Auth routes
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/refresh", authHandler.RefreshToken)

			// Protected auth routes
			authProtected := authGroup.Group("")
			authProtected.Use(middleware.Auth(authService))
			{
				authProtected.POST("/logout", authHandler.Logout)
				authProtected.GET("/me", authHandler.Me)
			}
		}

		// Notes routes (Protected)
		notesGroup := v1.Group("/notes")
		notesGroup.Use(middleware.Auth(authService))
		{
			notesGroup.POST("", noteHandler.CreateNote)
			notesGroup.GET("", noteHandler.ListNotes)
			notesGroup.GET("/:id", noteHandler.GetNote)
			notesGroup.PUT("/:id", noteHandler.UpdateNote)
			notesGroup.DELETE("/:id", noteHandler.DeleteNote)

			// Sharing and authorization checks
			notesGroup.POST("/:id/share", noteHandler.ShareNote)
			notesGroup.GET("/:id/shares", noteHandler.GetNoteShares)
			notesGroup.DELETE("/:id/share/:userId", noteHandler.UnshareNote)
			notesGroup.GET("/:id/access", noteHandler.CheckAccess)
		}
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
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

	log.Printf("notes-api-gin successfully running on :%s", cfg.Port)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down notes-api-gin...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("notes-api-gin exited cleanly")
}


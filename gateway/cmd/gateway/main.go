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

	"github.com/streed/smol-cluster/gateway/internal/auth"
	"github.com/streed/smol-cluster/gateway/internal/config"
	"github.com/streed/smol-cluster/gateway/internal/db"
	"github.com/streed/smol-cluster/gateway/internal/handlers"
	"github.com/streed/smol-cluster/gateway/internal/k8s"
	"github.com/streed/smol-cluster/gateway/internal/models"
	"github.com/streed/smol-cluster/gateway/internal/slack"
	"github.com/streed/smol-cluster/gateway/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	// Connect to database
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations
	if err := db.RunMigrations(ctx, pool); err != nil {
		log.Printf("warning: migration error (may be OK if already applied): %v", err)
	}

	queries := db.NewQueries(pool)

	// Create default admin user if configured and no users exist
	createDefaultAdmin(queries)

	// Initialize K8s client
	k8sClient, err := k8s.NewClient(cfg)
	if err != nil {
		log.Printf("warning: k8s client init failed (OK for local dev without k8s): %v", err)
	}

	// Initialize WebSocket hub
	hub := ws.NewHub()

	// Initialize Slack bot
	slackBot := slack.NewBot(cfg, queries, hub)

	// Setup routes
	deps := &handlers.Deps{
		Config:  cfg,
		Queries: queries,
		K8s:     k8sClient,
		Hub:     hub,
	}
	router := handlers.SetupRoutes(deps)

	// Start HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start Slack bot in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := slackBot.Start(ctx); err != nil {
			log.Printf("slack bot error: %v", err)
		}
	}()

	// Start server
	go func() {
		log.Printf("gateway listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced shutdown: %v", err)
	}

	log.Println("server stopped")
}

func createDefaultAdmin(queries *db.Queries) {
	email := os.Getenv("ADMIN_EMAIL")
	password := os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		return
	}

	count, err := queries.CountUsers(context.Background())
	if err != nil || count > 0 {
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Printf("failed to hash admin password: %v", err)
		return
	}

	_, err = queries.CreateUser(context.Background(), models.User{
		Email:        email,
		PasswordHash: hash,
		Name:         "Admin",
		Role:         "admin",
	})
	if err != nil {
		log.Printf("failed to create default admin: %v", err)
		return
	}

	log.Printf("created default admin user: %s", email)
}

package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/streed/smol-gang/gateway/internal/config"
	"github.com/streed/smol-gang/gateway/internal/db"
	"github.com/streed/smol-gang/gateway/internal/k8s"
	"github.com/streed/smol-gang/gateway/internal/llm"
	"github.com/streed/smol-gang/gateway/internal/middleware"
	"github.com/streed/smol-gang/gateway/internal/ws"
	"k8s.io/client-go/kubernetes"
)

type Deps struct {
	Config  *config.Config
	Queries *db.Queries
	K8s     *k8s.Client
	Hub     *ws.Hub
	LLM     *llm.Client
}

func SetupRoutes(deps *Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestIDMiddleware)
	r.Use(middleware.PrometheusMetrics)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	authHandler := &AuthHandler{Queries: deps.Queries, Config: deps.Config}
	githubWebhookHandler := &GitHubWebhookHandler{Config: deps.Config, Queries: deps.Queries, K8s: deps.K8s}
	userHandler := &UserHandler{Queries: deps.Queries}
	repoHandler := &RepoHandler{Queries: deps.Queries, Config: deps.Config}
	workstreamHandler := &WorkstreamHandler{Queries: deps.Queries, K8s: deps.K8s, Config: deps.Config, Hub: deps.Hub}
	auditHandler := &AuditHandler{Queries: deps.Queries}
	slackHandler := &SlackHandler{Config: deps.Config, Queries: deps.Queries, K8s: deps.K8s}
	var k8sClientset kubernetes.Interface
	if deps.K8s != nil {
		k8sClientset = deps.K8s.Clientset()
	}
	planHandler := &PlanHandler{Queries: deps.Queries, LLM: deps.LLM, K8s: k8sClientset, Config: deps.Config}

	r.Route("/api/v1", func(r chi.Router) {
		// Public routes (GitHub OAuth only)
		r.Group(func(r chi.Router) {
			r.Get("/auth/github", authHandler.GitHubLogin)
			r.Get("/auth/github/callback", authHandler.GitHubCallback)
		})

		// Slack webhook routes (verified via Slack signing secret, not JWT)
		r.Group(func(r chi.Router) {
			r.Post("/slack/command", slackHandler.HandleSlashCommand)
			r.Post("/slack/interact", slackHandler.HandleInteraction)
		})

		// GitHub webhook (verified via HMAC signature, not JWT)
		r.Post("/github/webhooks", githubWebhookHandler.HandleWebhook)

		// Internal routes (agent callbacks)
		r.Group(func(r chi.Router) {
			r.Post("/internal/workstreams/{id}/agent-message", workstreamHandler.AgentMessage)
			r.Post("/internal/workstreams/{id}/status", workstreamHandler.AgentStatusUpdate)
		})

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(deps.Config.JWTSecret))
			r.Use(middleware.AuditMiddleware(deps.Queries))

			// Auth
			r.Get("/auth/me", authHandler.Me)
			r.Post("/auth/refresh", authHandler.Refresh)

			// Repositories (read for all, write for operator+)
			r.Get("/repositories", repoHandler.List)
			r.Get("/repositories/{id}", repoHandler.Get)
			r.Get("/repositories/{id}/branches", repoHandler.ListBranches)

			// Workstreams (read for all)
			r.Get("/workstreams", workstreamHandler.List)
			r.Get("/workstreams/{id}", workstreamHandler.Get)
			r.Get("/workstreams/{id}/messages", workstreamHandler.GetMessages)
			r.Get("/workstreams/{id}/logs", workstreamHandler.GetLogs)
			r.Get("/workstreams/{id}/ports", workstreamHandler.GetPorts)
			r.Get("/workstreams/{id}/diff", workstreamHandler.GetDiff)

			// WebSocket
			r.Get("/ws/workstreams/{id}", func(w http.ResponseWriter, r *http.Request) {
				wsID := chi.URLParam(r, "id")
				deps.Hub.HandleWebSocket(w, r, wsID)
			})

			// Operator+ routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.RBACMiddleware("admin", "operator"))

				// Terminal - interactive shell (operator+ only)
				r.Get("/ws/workstreams/{id}/terminal", workstreamHandler.Terminal)
				r.Get("/ws/workstreams/{id}/terminal/{container}", workstreamHandler.Terminal)

				r.Get("/github/repos", repoHandler.ListGitHubRepos)
				r.Post("/github/repos/import", repoHandler.ImportGitHubRepo)

				r.Post("/repositories", repoHandler.Create)
				r.Put("/repositories/{id}", repoHandler.Update)
				r.Post("/repositories/{id}/sync-config", repoHandler.SyncConfig)

				r.Post("/workstreams", workstreamHandler.Create)
				r.Post("/workstreams/{id}/message", workstreamHandler.SendMessage)
				r.Post("/workstreams/{id}/complete", workstreamHandler.Complete)
				r.Post("/workstreams/{id}/cancel", workstreamHandler.Cancel)

				// Plans (DAG orchestrator)
				r.Get("/plans", planHandler.List)
				r.Post("/plans", planHandler.Create)
				r.Route("/plans/{planID}", func(r chi.Router) {
					r.Get("/", planHandler.Get)
					r.Delete("/", planHandler.Delete)
					r.Post("/tasks", planHandler.AddTask)
					r.Put("/tasks/{taskID}", planHandler.UpdateTask)
					r.Delete("/tasks/{taskID}", planHandler.RemoveTask)
					r.Post("/approve", planHandler.Approve)
					r.Post("/reject", planHandler.Reject)
					r.Post("/complete", planHandler.Complete)
					r.Post("/restart", planHandler.Restart)
					r.Put("/conversations", planHandler.SaveConversations)
				})
			})

			// Admin routes
			r.Group(func(r chi.Router) {
				r.Use(middleware.RBACMiddleware("admin"))

				r.Get("/users", userHandler.List)
				r.Get("/users/{id}", userHandler.Get)
				r.Put("/users/{id}", userHandler.Update)
				r.Delete("/users/{id}", userHandler.Delete)

				r.Delete("/repositories/{id}", repoHandler.Delete)
				r.Delete("/workstreams/{id}", workstreamHandler.Delete)

				r.Get("/audit-logs", auditHandler.List)
			})
		})
	})

	// Health check
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	return r
}

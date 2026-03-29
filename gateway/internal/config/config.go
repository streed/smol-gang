package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
	JWTSecret   string
	K8sNamespace string

	GitHubAppID         string
	GitHubAppPrivateKey string
	GitHubWebhookSecret string

	SlackBotToken      string
	SlackSigningSecret string
	SlackDefaultChannel string

	LLMApiURL   string
	LLMApiKey   string
	LLMModel    string
	LLMProvider string

	GitHubClientID     string
	GitHubClientSecret string
	FrontendURL        string

	AgentImage    string
	AppRunnerImage string
	LogLevel      string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		Port:                getEnvDefault("PORT", "8080"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		K8sNamespace:        getEnvDefault("K8S_NAMESPACE", "smol-gang"),
		GitHubAppID:         os.Getenv("GITHUB_APP_ID"),
		GitHubAppPrivateKey: os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		GitHubWebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		SlackBotToken:       os.Getenv("SLACK_BOT_TOKEN"),
		SlackSigningSecret:  os.Getenv("SLACK_SIGNING_SECRET"),
		SlackDefaultChannel: getEnvDefault("SLACK_DEFAULT_CHANNEL", "smol-gang"),
		LLMApiURL:           os.Getenv("LLM_API_URL"),
		LLMApiKey:           os.Getenv("LLM_API_KEY"),
		LLMModel:            os.Getenv("LLM_MODEL"),
		LLMProvider:         getEnvDefault("LLM_PROVIDER", "ollama-api"),
		GitHubClientID:      os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:  os.Getenv("GITHUB_CLIENT_SECRET"),
		FrontendURL:         getEnvDefault("FRONTEND_URL", "http://localhost:5000"),
		AgentImage:          getEnvDefault("AGENT_IMAGE", "smol-gang/agent:latest"),
		AppRunnerImage:      getEnvDefault("APP_RUNNER_IMAGE", "smol-gang/agent:latest-apprunner"),
		LogLevel:            getEnvDefault("LOG_LEVEL", "info"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func getEnvDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

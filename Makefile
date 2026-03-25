.PHONY: help dev dev-down build test clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev: ## Start full local development environment
	./scripts/dev-setup.sh

dev-down: ## Stop local development environment
	./scripts/dev-teardown.sh

dev-compose: ## Start docker-compose services only (no K8s)
	docker compose up -d --build

dev-compose-down: ## Stop docker-compose services
	docker compose down

build: build-gateway build-web build-agent ## Build all Docker images

build-gateway: ## Build gateway Docker image
	docker build -t smol-cluster/gateway:latest ./gateway

build-web: ## Build web UI Docker image
	docker build -t smol-cluster/web:latest ./web

build-agent: ## Build agent Docker image
	docker build -t smol-cluster/agent:latest ./agent

test: test-gateway ## Run all tests

test-gateway: ## Run gateway tests
	cd gateway && go test ./...

migrate: ## Run database migrations
	docker compose exec gateway /app/gateway migrate

seed: ## Seed test data
	./scripts/seed-data.sh

lint: ## Run linters
	cd gateway && go vet ./...
	cd web && npm run lint 2>/dev/null || true

clean: ## Clean up everything
	docker compose down -v
	kind delete cluster --name smol-cluster 2>/dev/null || true
	docker rmi smol-cluster/gateway:latest smol-cluster/web:latest smol-cluster/agent:latest 2>/dev/null || true

kind-setup: ## Create Kind K8s cluster
	./scripts/kind-setup.sh

kind-teardown: ## Delete Kind K8s cluster
	kind delete cluster --name smol-cluster

kind-load: build ## Build and load images into Kind
	kind load docker-image smol-cluster/gateway:latest --name smol-cluster
	kind load docker-image smol-cluster/web:latest --name smol-cluster
	kind load docker-image smol-cluster/agent:latest --name smol-cluster

helm-install: ## Install Helm chart to Kind cluster
	helm upgrade --install smol-cluster ./helm/smol-cluster \
		--namespace smol-cluster --create-namespace \
		--set gateway.image.repository=smol-cluster/gateway \
		--set gateway.image.tag=latest \
		--set gateway.image.pullPolicy=Never

helm-uninstall: ## Uninstall Helm chart
	helm uninstall smol-cluster --namespace smol-cluster

logs-gateway: ## Tail gateway logs
	docker compose logs -f gateway

logs-web: ## Tail web UI logs
	docker compose logs -f web

logs-db: ## Tail database logs
	docker compose logs -f postgres

# smol-gang

A Kubernetes-based system that orchestrates multiple [smolagent](https://github.com/huggingface/smolagents) instances to work on features in parallel across one or many repositories.

![Login Page](docs/screenshots/login-page.png)

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌──────────────────────────────────┐
│   Web UI    │────▶│   Gateway    │────▶│   Kubernetes (Kind / Production) │
│  (React)    │     │   (Go/Chi)   │     │                                  │
└─────────────┘     └──────┬───────┘     │  ┌─────────────────────────────┐ │
                           │             │  │  Agent Pod (per workstream) │ │
┌─────────────┐            │             │  │  ┌───────┐ ┌─────┐ ┌─────┐ │ │
│   Slack     │────────────┘             │  │  │ Agent │ │ App │ │DinD │ │ │
│ Integration │                          │  │  └───────┘ └─────┘ └─────┘ │ │
└─────────────┘                          │  └─────────────────────────────┘ │
                                         └──────────────────────────────────┘
```

Each workstream runs as a 3-container Kubernetes pod:
- **Agent** — smolagent + bridge server that receives tasks and sends results
- **App** — runs the application under development (with port forwarding)
- **DinD** — Docker-in-Docker sidecar for builds and container operations

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (with Docker Compose)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) (Kubernetes in Docker)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/) (v3+)
- [Go](https://go.dev/dl/) 1.22+ (for gateway development)
- [Node.js](https://nodejs.org/) 18+ (for web UI development)
- An LLM API endpoint (OpenAI-compatible — local Ollama, OpenAI, Anthropic via proxy, etc.)

## Quick Start

### 1. Clone and configure

```bash
git clone https://github.com/streed/smol-cluster-.git
cd smol-cluster-
cp .env.example .env
```

Edit `.env` with your settings:

```bash
# Point to your LLM API (default: local Ollama)
LLM_API_URL=http://host.docker.internal:11434/v1
LLM_API_KEY=           # leave empty for Ollama
LLM_MODEL=llama3       # or gpt-4, claude-3-sonnet, etc.

# Optional integrations
GITHUB_APP_ID=
SLACK_BOT_TOKEN=
SLACK_SIGNING_SECRET=

# Auth (change in production!)
JWT_SECRET=dev-jwt-secret-change-in-production
ADMIN_EMAIL=admin@localhost
ADMIN_PASSWORD=admin
```

### 2. Start everything

```bash
make dev
```

This single command will:
1. Create `.env` from `.env.example` if it doesn't exist
2. Start Docker Compose services (PostgreSQL, Gateway, Web UI)
3. Create a Kind Kubernetes cluster (1 control-plane + 2 workers)
4. Install the Helm chart into the Kind cluster

### 3. Access the application

| Service     | URL                        |
|-------------|----------------------------|
| Web UI      | http://localhost:3000       |
| Gateway API | http://localhost:8080       |
| PostgreSQL  | localhost:5432              |

**Default login credentials:**
- Email: `admin@localhost`
- Password: `admin`

## Development

### Available Make targets

```bash
make help                # Show all available targets
```

| Target              | Description                                     |
|---------------------|-------------------------------------------------|
| `make dev`          | Start full local dev environment                |
| `make dev-down`     | Stop everything                                 |
| `make dev-compose`  | Start Docker Compose only (no K8s)              |
| `make dev-compose-down` | Stop Docker Compose services                |
| `make build`        | Build all Docker images                         |
| `make build-gateway`| Build gateway image                             |
| `make build-web`    | Build web UI image                              |
| `make build-agent`  | Build agent image                               |
| `make test`         | Run all tests                                   |
| `make test-gateway` | Run gateway Go tests                            |
| `make lint`         | Run linters                                     |
| `make migrate`      | Run database migrations                         |
| `make seed`         | Seed test data                                  |
| `make kind-setup`   | Create Kind K8s cluster                         |
| `make kind-teardown`| Delete Kind K8s cluster                         |
| `make kind-load`    | Build and load images into Kind                 |
| `make helm-install` | Install Helm chart to Kind cluster              |
| `make helm-uninstall`| Uninstall Helm chart                           |
| `make logs-gateway` | Tail gateway logs                               |
| `make logs-web`     | Tail web UI logs                                |
| `make logs-db`      | Tail database logs                              |
| `make clean`        | Remove everything (containers, images, cluster) |

### Working on the Gateway (Go)

```bash
cd gateway
go test ./...          # Run tests
go vet ./...           # Run linter
go build ./cmd/gateway # Build binary
```

The gateway uses:
- [Chi](https://github.com/go-chi/chi) router with middleware chain
- PostgreSQL via `pgxpool`
- Kubernetes client-go for pod management
- JWT authentication with role-based access (admin, operator, user)
- WebSocket hub for real-time messaging
- Prometheus metrics at `/metrics`

### Working on the Web UI (React/TypeScript)

```bash
cd web
npm install
npm run dev            # Start Vite dev server (hot reload)
npm run build          # Production build
npm run lint           # ESLint
```

The web UI features a cyberpunk/hacker theme with neon colors, JetBrains Mono font, and glass morphism effects.

### Working on the Agent

The agent runs inside Kubernetes pods. For local testing:

```bash
cd agent
docker build -t smol-gang/agent:latest .
docker build -t smol-gang/agent:latest-apprunner -f Dockerfile.apprunner .
```

Agent components:
- `scripts/entrypoint.sh` — orchestrates clone, setup, smolagent startup, and bridge
- `scripts/bridge.py` — HTTP bridge between gateway and smolagent ACP
- `scripts/cleanup.sh` — commits, pushes, and reports final status
- `scripts/setup.sh` — runs repo-specific setup commands
- `scripts/app-entrypoint.sh` — starts the application container

### Docker Compose Only (no Kubernetes)

If you don't need Kubernetes/Kind and just want to run the web UI and gateway:

```bash
make dev-compose       # Start postgres + gateway + web
make dev-compose-down  # Stop
```

This is useful for frontend development where you don't need to launch agent pods.

### Kubernetes Debugging

```bash
# Check pod status
kubectl get pods -n smol-gang

# Check services
kubectl get svc -n smol-gang

# Gateway logs in K8s
kubectl logs -n smol-gang -l app.kubernetes.io/component=gateway -f

# Agent pod logs
kubectl logs -n smol-gang <pod-name> -c agent -f

# Access a pod shell
kubectl exec -it -n smol-gang <pod-name> -c app -- /bin/sh
```

## Observability

An optional observability stack is included:

```bash
cd observability
docker compose -f docker-compose.observability.yml up -d
```

| Service    | URL                    |
|------------|------------------------|
| Prometheus | http://localhost:9090   |
| Grafana    | http://localhost:3001   |
| Loki       | http://localhost:3100   |

Grafana comes pre-configured with Prometheus and Loki datasources and smol-gang dashboards.

## Slack Integration

smol-gang supports Slack slash commands for managing workstreams. Set up:

1. Create a Slack App at https://api.slack.com/apps
2. Add a Slash Command: `/smol` pointing to `https://your-gateway/api/v1/slack/command`
3. Enable Interactivity with URL: `https://your-gateway/api/v1/slack/interact`
4. Add `SLACK_BOT_TOKEN` and `SLACK_SIGNING_SECRET` to your `.env`

Available slash commands:
- `/smol list` — list active workstreams
- `/smol status <name>` — get workstream status
- `/smol start <repo> <branch> <prompt>` — start a new workstream
- `/smol message <name> <message>` — send a message to an agent
- `/smol complete <name>` — mark a workstream as complete
- `/smol cancel <name>` — cancel a workstream
- `/smol repos` — list registered repositories
- `/smol help` — show help

## Helm Chart

For production-like deployments:

```bash
helm upgrade --install smol-gang ./helm/smol-cluster \
  --namespace smol-gang --create-namespace \
  -f helm/smol-cluster/values.yaml \
  --set secrets.jwtSecret="your-secret" \
  --set secrets.databasePassword="your-password"
```

The chart includes: gateway, PostgreSQL (embedded or external), RBAC, ingress, network policies, and the full observability stack (Prometheus, Loki, Grafana, Promtail).

## Project Structure

```
smol-cluster-/
├── agent/                    # Agent container
│   ├── Dockerfile           # Agent image (smolagent + bridge)
│   ├── Dockerfile.apprunner # App runner sidecar image
│   └── scripts/             # Entrypoint, bridge, cleanup scripts
├── gateway/                  # Go API server
│   ├── cmd/gateway/         # Main entry point
│   └── internal/            # Handlers, K8s client, middleware, models
├── web/                      # React/TypeScript frontend
│   └── src/
│       ├── pages/           # LoginPage, Dashboard, Workstreams, etc.
│       ├── components/      # ChatInterface, Terminal, Layout, etc.
│       ├── hooks/           # useWebSocket
│       ├── store/           # Zustand auth store
│       └── api/             # Axios endpoints
├── helm/smol-cluster/        # Helm chart (smol-gang)
├── observability/            # Prometheus/Loki/Grafana docker-compose
├── scripts/                  # Dev setup, Kind config, seed data
├── docker-compose.yml        # Local development services
├── Makefile                  # Build/dev automation
└── .env.example              # Environment template
```

## License

See [LICENSE](LICENSE) for details.

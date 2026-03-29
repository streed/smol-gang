# Contributing to smol-gang

Thanks for your interest in contributing! This document covers how to get started.

## Development Setup

1. Install prerequisites: Docker, Kind, kubectl, Helm, Go 1.22+, Node.js 18+
2. Clone the repo and run `make dev` to start the full local environment
3. See the [README](README.md) for detailed setup instructions

## Project Structure

- **gateway/** — Go API server (Chi router, PostgreSQL, K8s client)
- **web/** — React/TypeScript frontend (Vite, Tailwind CSS)
- **agent/** — Agent container (smolagent + ACP bridge)
- **helm/** — Helm chart for Kubernetes deployment
- **scripts/** — Development and setup scripts
- **observability/** — Prometheus, Grafana, Loki stack

## Making Changes

### Gateway (Go)

```bash
cd gateway
go test ./...    # Run tests
go vet ./...     # Lint
go build ./cmd/gateway
```

### Web UI (React/TypeScript)

```bash
cd web
npm install
npm run dev      # Dev server with hot reload
npm run lint     # ESLint
npm run build    # Production build
```

### Agent

The agent runs inside Kubernetes pods. Build images locally:

```bash
make build-agent
```

## Pull Request Guidelines

- Keep PRs focused on a single change
- Include a clear description of what changed and why
- Add tests for new functionality
- Run `make test` and `make lint` before submitting
- Update documentation if your change affects user-facing behavior

## Code Style

- **Go**: Follow standard Go conventions. Run `go vet` and `go fmt`.
- **TypeScript/React**: Follow the existing patterns. Run `npm run lint`.
- **Shell scripts**: Use `set -euo pipefail` and add comments for non-obvious logic.

## Reporting Issues

Open an issue on GitHub with:
- A clear description of the problem or feature request
- Steps to reproduce (for bugs)
- Expected vs actual behavior
- Your environment (OS, Docker version, K8s version)

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

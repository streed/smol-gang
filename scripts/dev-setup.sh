#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ------------------------------------------------------------------
# Check prerequisites
# ------------------------------------------------------------------
check_prereqs() {
    local missing=()
    for cmd in docker kind kubectl helm curl; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    if [[ ${#missing[@]} -gt 0 ]]; then
        error "Missing required tools: ${missing[*]}\nPlease install them before continuing."
    fi

    if ! docker info &>/dev/null; then
        error "Docker daemon is not running. Please start Docker first."
    fi

    info "All prerequisites satisfied."
}

# ------------------------------------------------------------------
# Create .env from .env.example
# ------------------------------------------------------------------
setup_env() {
    if [[ ! -f "${PROJECT_DIR}/.env" ]]; then
        if [[ -f "${PROJECT_DIR}/.env.example" ]]; then
            cp "${PROJECT_DIR}/.env.example" "${PROJECT_DIR}/.env"
            info "Created .env from .env.example. Edit it to customize settings."
        else
            warn ".env.example not found. Skipping .env creation."
        fi
    else
        info ".env already exists, skipping."
    fi
}

# ------------------------------------------------------------------
# Start docker-compose services
# ------------------------------------------------------------------
start_compose() {
    info "Starting docker-compose services (postgres, gateway, web)..."
    cd "${PROJECT_DIR}"
    docker compose up -d --build

    info "Waiting for postgres to be healthy..."
    local retries=30
    while [[ $retries -gt 0 ]]; do
        if docker compose exec -T postgres pg_isready -U smol -d smol_cluster &>/dev/null; then
            info "Postgres is healthy."
            break
        fi
        retries=$((retries - 1))
        sleep 2
    done
    if [[ $retries -eq 0 ]]; then
        error "Postgres did not become healthy in time."
    fi

    info "Waiting for gateway to be ready..."
    retries=30
    while [[ $retries -gt 0 ]]; do
        if curl -sf http://localhost:8080/healthz &>/dev/null 2>&1 || \
           curl -sf http://localhost:8080/health &>/dev/null 2>&1 || \
           curl -sf http://localhost:8080/ &>/dev/null 2>&1; then
            info "Gateway is ready."
            break
        fi
        retries=$((retries - 1))
        sleep 2
    done
    if [[ $retries -eq 0 ]]; then
        warn "Gateway health check timed out. It may still be starting. Check logs with: docker compose logs -f gateway"
    fi

    info "Waiting for web UI to be ready..."
    retries=20
    while [[ $retries -gt 0 ]]; do
        if curl -sf http://localhost:3000/ &>/dev/null 2>&1; then
            info "Web UI is ready."
            break
        fi
        retries=$((retries - 1))
        sleep 2
    done
    if [[ $retries -eq 0 ]]; then
        warn "Web UI health check timed out. It may still be starting. Check logs with: docker compose logs -f web"
    fi
}

# ------------------------------------------------------------------
# Set up Kind cluster
# ------------------------------------------------------------------
setup_kind() {
    info "Setting up Kind Kubernetes cluster..."
    bash "${SCRIPT_DIR}/kind-setup.sh"
}

# ------------------------------------------------------------------
# Install Helm chart pointing to local postgres
# ------------------------------------------------------------------
install_helm_chart() {
    info "Installing Helm chart into Kind cluster..."

    # Get the docker bridge IP so K8s pods can reach the host docker-compose postgres
    local host_ip
    host_ip=$(docker network inspect bridge --format '{{range .IPAM.Config}}{{.Gateway}}{{end}}' 2>/dev/null || echo "172.17.0.1")

    helm upgrade --install smol-cluster "${PROJECT_DIR}/helm/smol-cluster" \
        --namespace smol-cluster \
        --create-namespace \
        --set gateway.image.repository=smol-cluster/gateway \
        --set gateway.image.tag=latest \
        --set gateway.image.pullPolicy=Never \
        --set gateway.env.DATABASE_URL="postgres://smol:smol_dev_password@${host_ip}:5432/smol_cluster?sslmode=disable" \
        --set gateway.env.K8S_IN_CLUSTER="true" \
        --set gateway.env.K8S_NAMESPACE="smol-cluster" \
        --set gateway.env.LOG_LEVEL="debug"

    info "Helm chart installed. Waiting for gateway pod..."
    kubectl wait --namespace smol-cluster \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/name=gateway \
        --timeout=120s 2>/dev/null || \
        warn "Gateway pod not ready yet. Check with: kubectl get pods -n smol-cluster"
}

# ------------------------------------------------------------------
# Print summary
# ------------------------------------------------------------------
print_summary() {
    echo ""
    info "============================================="
    info "  smol-cluster dev environment is running!"
    info "============================================="
    echo ""
    echo "  Access URLs:"
    echo "    Web UI:   http://localhost:3000"
    echo "    Gateway:  http://localhost:8080"
    echo "    Postgres: localhost:5432"
    echo ""
    echo "  Default credentials:"
    echo "    Email:    admin@localhost"
    echo "    Password: admin"
    echo ""
    echo "  Useful commands:"
    echo "    make logs-gateway      # Tail gateway logs"
    echo "    make logs-web          # Tail web UI logs"
    echo "    make seed              # Seed test data"
    echo "    make dev-down          # Stop everything"
    echo ""
    echo "  Kubernetes:"
    echo "    kubectl get pods -n smol-cluster"
    echo "    kubectl get svc  -n smol-cluster"
    echo ""
}

# ------------------------------------------------------------------
# Main
# ------------------------------------------------------------------
main() {
    info "=== smol-cluster Development Environment Setup ==="
    check_prereqs
    setup_env
    start_compose
    setup_kind
    install_helm_chart
    print_summary
}

main "$@"

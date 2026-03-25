#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CLUSTER_NAME="${KIND_CLUSTER_NAME:-smol-cluster}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }

CLEAN=false

# Parse flags
while [[ $# -gt 0 ]]; do
    case "$1" in
        --clean)
            CLEAN=true
            shift
            ;;
        *)
            echo "Usage: $0 [--clean]"
            echo "  --clean    Remove Docker volumes and all local data"
            exit 1
            ;;
    esac
done

# ------------------------------------------------------------------
# Stop docker-compose
# ------------------------------------------------------------------
info "Stopping docker-compose services..."
cd "${PROJECT_DIR}"
if [[ "$CLEAN" == "true" ]]; then
    docker compose down -v
    info "Docker-compose stopped and volumes removed."
else
    docker compose down
    info "Docker-compose stopped (volumes preserved)."
fi

# ------------------------------------------------------------------
# Delete Kind cluster
# ------------------------------------------------------------------
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    info "Deleting Kind cluster '${CLUSTER_NAME}'..."
    kind delete cluster --name "${CLUSTER_NAME}"
    info "Kind cluster deleted."
else
    info "Kind cluster '${CLUSTER_NAME}' not found, skipping."
fi

# ------------------------------------------------------------------
# Optionally remove images
# ------------------------------------------------------------------
if [[ "$CLEAN" == "true" ]]; then
    info "Removing locally built Docker images..."
    docker rmi smol-cluster/gateway:latest smol-cluster/web:latest smol-cluster/agent:latest 2>/dev/null || true
    info "Cleanup complete."
fi

echo ""
info "Development environment torn down."
if [[ "$CLEAN" == "false" ]]; then
    echo "  Tip: Run with --clean to also remove Docker volumes and images."
fi

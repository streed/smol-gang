#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
CLUSTER_NAME="${KIND_CLUSTER_NAME:-smol-gang}"

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
    for cmd in kind kubectl docker helm; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    if [[ ${#missing[@]} -gt 0 ]]; then
        error "Missing required tools: ${missing[*]}\nPlease install them before continuing."
    fi
    info "All prerequisites found."
}

# ------------------------------------------------------------------
# Create Kind cluster
# ------------------------------------------------------------------
create_cluster() {
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        warn "Kind cluster '${CLUSTER_NAME}' already exists. Skipping creation."
        kubectl cluster-info --context "kind-${CLUSTER_NAME}" &>/dev/null || \
            error "Cluster exists but is not reachable. Delete it with: kind delete cluster --name ${CLUSTER_NAME}"
        return 0
    fi

    info "Creating Kind cluster '${CLUSTER_NAME}'..."
    kind create cluster --config "${SCRIPT_DIR}/kind-config.yaml" --image kindest/node:v1.33.1 --wait 180s
    info "Kind cluster created successfully."
}

# ------------------------------------------------------------------
# Create namespace
# ------------------------------------------------------------------
create_namespace() {
    if kubectl get namespace smol-gang &>/dev/null; then
        info "Namespace 'smol-gang' already exists."
    else
        info "Creating namespace 'smol-gang'..."
        kubectl create namespace smol-gang
    fi
}

# ------------------------------------------------------------------
# Install nginx ingress controller
# ------------------------------------------------------------------
install_ingress() {
    info "Installing nginx ingress controller..."
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

    info "Waiting for ingress controller to be ready..."
    kubectl wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=120s || warn "Ingress controller not ready yet; it may still be starting."
}

# ------------------------------------------------------------------
# Build and load Docker images into Kind
# ------------------------------------------------------------------
build_and_load_images() {
    info "Building Docker images..."

    if [[ -f "${PROJECT_DIR}/gateway/Dockerfile" ]]; then
        info "Building gateway image..."
        docker build -t smol-gang/gateway:latest "${PROJECT_DIR}/gateway"
    else
        warn "gateway/Dockerfile not found, skipping gateway build."
    fi

    if [[ -f "${PROJECT_DIR}/web/Dockerfile" ]]; then
        info "Building web image..."
        docker build -t smol-gang/web:latest "${PROJECT_DIR}/web"
    else
        warn "web/Dockerfile not found, skipping web build."
    fi

    if [[ -f "${PROJECT_DIR}/agent/Dockerfile" ]]; then
        info "Building agent image..."
        docker build -t smol-gang/agent:latest "${PROJECT_DIR}/agent"
    else
        warn "agent/Dockerfile not found, skipping agent build."
    fi

    info "Loading images into Kind cluster..."
    for image in smol-gang/gateway:latest smol-gang/web:latest smol-gang/agent:latest; do
        if docker image inspect "$image" &>/dev/null; then
            kind load docker-image "$image" --name "${CLUSTER_NAME}"
            info "Loaded ${image}"
        else
            warn "Image ${image} not found locally, skipping load."
        fi
    done
}

# ------------------------------------------------------------------
# Apply RBAC manifests
# ------------------------------------------------------------------
apply_rbac() {
    local rbac_file="${PROJECT_DIR}/helm/smol-gang/templates/rbac.yaml"
    if [[ -f "$rbac_file" ]]; then
        info "Applying RBAC manifests..."
        # Use helm template to render, then apply
        helm template smol-gang "${PROJECT_DIR}/helm/smol-gang" \
            --namespace smol-gang \
            --show-only templates/rbac.yaml | kubectl apply -f - 2>/dev/null || \
        warn "Could not apply RBAC via helm template. You may need to run 'make helm-install' instead."
    else
        warn "RBAC manifest not found at ${rbac_file}, skipping."
    fi
}

# ------------------------------------------------------------------
# Main
# ------------------------------------------------------------------
main() {
    info "=== smol-gang Kind Setup ==="
    check_prereqs
    create_cluster
    create_namespace
    install_ingress
    build_and_load_images
    apply_rbac

    echo ""
    info "=== Kind cluster '${CLUSTER_NAME}' is ready! ==="
    echo ""
    echo "  Useful commands:"
    echo "    kubectl get pods -n smol-gang          # List pods"
    echo "    kubectl get svc  -n smol-gang          # List services"
    echo "    kubectl logs -f <pod> -n smol-gang     # Tail pod logs"
    echo "    kind delete cluster --name ${CLUSTER_NAME}  # Tear down"
    echo ""
    echo "  To install the Helm chart:"
    echo "    make helm-install"
    echo ""
}

main "$@"

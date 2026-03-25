#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# ------------------------------------------------------------------
# Wait for API to be reachable
# ------------------------------------------------------------------
info "Waiting for API at ${API_URL}..."
retries=15
while [[ $retries -gt 0 ]]; do
    if curl -sf "${API_URL}/healthz" &>/dev/null 2>&1 || \
       curl -sf "${API_URL}/health" &>/dev/null 2>&1 || \
       curl -sf "${API_URL}/api/v1/health" &>/dev/null 2>&1; then
        break
    fi
    retries=$((retries - 1))
    sleep 2
done
if [[ $retries -eq 0 ]]; then
    warn "API health check timed out. Proceeding anyway..."
fi

# ------------------------------------------------------------------
# Register admin user
# ------------------------------------------------------------------
info "Registering admin user..."
REGISTER_RESPONSE=$(curl -sf -X POST "${API_URL}/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d '{
        "email": "admin@localhost",
        "password": "admin",
        "name": "Admin User"
    }' 2>/dev/null || echo '{"error":"may already exist"}')
echo "  Register response: ${REGISTER_RESPONSE}"

# ------------------------------------------------------------------
# Login and get token
# ------------------------------------------------------------------
info "Logging in as admin..."
LOGIN_RESPONSE=$(curl -sf -X POST "${API_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{
        "email": "admin@localhost",
        "password": "admin"
    }' 2>/dev/null || error "Failed to login. Is the gateway running?")

TOKEN=$(echo "${LOGIN_RESPONSE}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || \
        echo "${LOGIN_RESPONSE}" | jq -r '.token // empty' 2>/dev/null || \
        echo "")

if [[ -z "$TOKEN" ]]; then
    warn "Could not extract token from login response: ${LOGIN_RESPONSE}"
    warn "Continuing without authentication..."
    AUTH_HEADER=""
else
    info "Login successful. Token obtained."
    AUTH_HEADER="Authorization: Bearer ${TOKEN}"
fi

# ------------------------------------------------------------------
# Create operator user
# ------------------------------------------------------------------
info "Creating operator user..."
curl -sf -X POST "${API_URL}/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    ${AUTH_HEADER:+-H "$AUTH_HEADER"} \
    -d '{
        "email": "operator@localhost",
        "password": "operator",
        "name": "Operator User",
        "role": "operator"
    }' 2>/dev/null || warn "Operator user creation failed (may already exist)."
echo ""

# ------------------------------------------------------------------
# Create viewer user
# ------------------------------------------------------------------
info "Creating viewer user..."
curl -sf -X POST "${API_URL}/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    ${AUTH_HEADER:+-H "$AUTH_HEADER"} \
    -d '{
        "email": "viewer@localhost",
        "password": "viewer",
        "name": "Viewer User",
        "role": "viewer"
    }' 2>/dev/null || warn "Viewer user creation failed (may already exist)."
echo ""

# ------------------------------------------------------------------
# Link a sample repository
# ------------------------------------------------------------------
info "Linking sample repository..."
REPO_RESPONSE=$(curl -sf -X POST "${API_URL}/api/v1/repositories" \
    -H "Content-Type: application/json" \
    ${AUTH_HEADER:+-H "$AUTH_HEADER"} \
    -d '{
        "url": "https://github.com/smol-ai/developer",
        "name": "smol-developer",
        "default_branch": "main"
    }' 2>/dev/null || echo '{"error":"failed"}')
echo "  Repository response: ${REPO_RESPONSE}"

REPO_ID=$(echo "${REPO_RESPONSE}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || \
          echo "${REPO_RESPONSE}" | jq -r '.id // empty' 2>/dev/null || \
          echo "")

# ------------------------------------------------------------------
# Create a sample workstream
# ------------------------------------------------------------------
info "Creating sample workstream..."
WORKSTREAM_RESPONSE=$(curl -sf -X POST "${API_URL}/api/v1/workstreams" \
    -H "Content-Type: application/json" \
    ${AUTH_HEADER:+-H "$AUTH_HEADER"} \
    -d "{
        \"title\": \"Sample: Add unit tests\",
        \"description\": \"Add comprehensive unit tests to the project. Cover edge cases and ensure at least 80% code coverage.\",
        \"repository_id\": \"${REPO_ID:-}\",
        \"priority\": \"medium\"
    }" 2>/dev/null || echo '{"error":"failed"}')
echo "  Workstream response: ${WORKSTREAM_RESPONSE}"

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo ""
info "=== Seed data created ==="
echo ""
echo "  Users:"
echo "    admin@localhost    / admin     (admin role)"
echo "    operator@localhost / operator  (operator role)"
echo "    viewer@localhost   / viewer    (viewer role)"
echo ""
echo "  Repository: smol-developer (smol-ai/developer)"
echo "  Workstream: Sample: Add unit tests"
echo ""

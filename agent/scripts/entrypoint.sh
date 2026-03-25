#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# smol-cluster Agent Entrypoint
# Orchestrates repo clone, setup, agent bridge, and smolagent ACP server.
# =============================================================================

LOG_PREFIX="[smol-cluster-agent]"

log() {
    echo "${LOG_PREFIX} $(date -u '+%Y-%m-%dT%H:%M:%SZ') $*"
}

log_error() {
    echo "${LOG_PREFIX} $(date -u '+%Y-%m-%dT%H:%M:%SZ') ERROR: $*" >&2
}

# ---------------------------------------------------------------------------
# Required environment variables
# ---------------------------------------------------------------------------
: "${REPO_URL:?REPO_URL is required}"
: "${BRANCH_NAME:?BRANCH_NAME is required}"
: "${GIT_TOKEN:?GIT_TOKEN is required}"
: "${LLM_API_URL:?LLM_API_URL is required}"
: "${LLM_API_KEY:?LLM_API_KEY is required}"
: "${LLM_MODEL:?LLM_MODEL is required}"
: "${AGENT_PROMPT:?AGENT_PROMPT is required}"
: "${GATEWAY_URL:?GATEWAY_URL is required}"
: "${WORKSTREAM_ID:?WORKSTREAM_ID is required}"
: "${GATEWAY_TOKEN:?GATEWAY_TOKEN is required}"

# Optional environment variables with defaults
ACP_PORT="${ACP_PORT:-8021}"
BRIDGE_PORT="${BRIDGE_PORT:-8022}"
SETUP_COMMANDS="${SETUP_COMMANDS:-}"

WORKSPACE="/workspace/repo"
PIDS_FILE="/tmp/agent_pids"

# ---------------------------------------------------------------------------
# Cleanup handler — push changes & report on exit
# ---------------------------------------------------------------------------
cleanup() {
    local exit_code=$?
    log "Cleanup triggered (exit_code=${exit_code})"

    # Kill child processes
    if [[ -f "$PIDS_FILE" ]]; then
        while read -r pid; do
            kill "$pid" 2>/dev/null || true
        done < "$PIDS_FILE"
        rm -f "$PIDS_FILE"
    fi

    # Run the cleanup script to commit/push remaining changes
    if [[ -d "$WORKSPACE/.git" ]]; then
        /app/scripts/cleanup.sh "$WORKSPACE" "$BRANCH_NAME" "$GATEWAY_URL" "$WORKSTREAM_ID" "$GATEWAY_TOKEN" "$exit_code"
    fi

    log "Agent pod shutting down"
    exit "$exit_code"
}

trap cleanup EXIT INT TERM

# ---------------------------------------------------------------------------
# Report status back to gateway
# ---------------------------------------------------------------------------
report_status() {
    local status="$1"
    local message="${2:-}"

    curl -sf -X POST \
        "${GATEWAY_URL}/api/v1/internal/workstreams/${WORKSTREAM_ID}/agent-message" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
        -d "$(jq -n --arg s "$status" --arg m "$message" \
            '{source: "agent", content: ("[status:\(.s)] " + .m)}')" \
        2>/dev/null || log_error "Failed to report status: ${status}"
}

# ---------------------------------------------------------------------------
# Step 1: Clone the repository
# ---------------------------------------------------------------------------
log "Cloning repository: ${REPO_URL}"
report_status "initializing" "Cloning repository..."

# Insert token into the URL for HTTPS auth
AUTH_URL=$(echo "$REPO_URL" | sed "s|https://|https://x-access-token:${GIT_TOKEN}@|")

if ! git clone --depth=50 "$AUTH_URL" "$WORKSPACE" 2>&1; then
    log_error "Failed to clone repository"
    report_status "error" "Failed to clone repository"
    exit 1
fi

cd "$WORKSPACE"

# Configure git identity
git config user.email "agent@smol-cluster.local"
git config user.name "smol-cluster-agent"

# ---------------------------------------------------------------------------
# Step 2: Create and checkout the feature branch
# ---------------------------------------------------------------------------
log "Creating branch: ${BRANCH_NAME}"

# Check if the branch already exists on the remote
if git ls-remote --exit-code --heads origin "$BRANCH_NAME" >/dev/null 2>&1; then
    log "Branch ${BRANCH_NAME} exists on remote, checking out"
    git fetch origin "$BRANCH_NAME"
    git checkout "$BRANCH_NAME"
else
    log "Creating new branch ${BRANCH_NAME}"
    git checkout -b "$BRANCH_NAME"
fi

report_status "initializing" "Repository cloned, branch ready"

# ---------------------------------------------------------------------------
# Step 3: Read .smol-cluster.yaml and run setup
# ---------------------------------------------------------------------------
log "Running setup..."
report_status "initializing" "Running setup commands..."

/app/scripts/setup.sh "$WORKSPACE" "$SETUP_COMMANDS"
setup_exit=$?

if [[ $setup_exit -ne 0 ]]; then
    log_error "Setup failed with exit code ${setup_exit}"
    report_status "error" "Setup commands failed"
    exit 1
fi

report_status "initializing" "Setup complete"

# ---------------------------------------------------------------------------
# Step 4: Start smolagent in ACP mode
# ---------------------------------------------------------------------------
log "Starting smolagent ACP server on port ${ACP_PORT}"

export LITELLM_API_BASE="$LLM_API_URL"
export LITELLM_API_KEY="$LLM_API_KEY"
export LITELLM_MODEL="$LLM_MODEL"

# Start smolagent ACP server in the background
smolagent \
    --model-type "LiteLLMModel" \
    --model-id "$LLM_MODEL" \
    --port "$ACP_PORT" \
    --host "0.0.0.0" \
    > /tmp/smolagent.log 2>&1 &

SMOLAGENT_PID=$!
echo "$SMOLAGENT_PID" > "$PIDS_FILE"
log "smolagent started with PID ${SMOLAGENT_PID}"

# Wait for smolagent to become healthy
RETRIES=0
MAX_RETRIES=30
while [[ $RETRIES -lt $MAX_RETRIES ]]; do
    if curl -sf "http://127.0.0.1:${ACP_PORT}/" >/dev/null 2>&1; then
        log "smolagent ACP server is ready"
        break
    fi

    # Check that the process is still alive
    if ! kill -0 "$SMOLAGENT_PID" 2>/dev/null; then
        log_error "smolagent process died during startup"
        log_error "smolagent log:"
        cat /tmp/smolagent.log >&2
        report_status "error" "smolagent failed to start"
        exit 1
    fi

    RETRIES=$((RETRIES + 1))
    sleep 2
done

if [[ $RETRIES -ge $MAX_RETRIES ]]; then
    log_error "smolagent did not become ready in time"
    cat /tmp/smolagent.log >&2
    report_status "error" "smolagent startup timeout"
    exit 1
fi

# ---------------------------------------------------------------------------
# Step 5: Start the bridge server
# ---------------------------------------------------------------------------
log "Starting bridge server on port ${BRIDGE_PORT}"

python3 /app/scripts/bridge.py \
    --port "$BRIDGE_PORT" \
    --acp-port "$ACP_PORT" \
    --gateway-url "$GATEWAY_URL" \
    --workstream-id "$WORKSTREAM_ID" \
    --gateway-token "$GATEWAY_TOKEN" \
    --workspace "$WORKSPACE" \
    --branch "$BRANCH_NAME" \
    > /tmp/bridge.log 2>&1 &

BRIDGE_PID=$!
echo "$BRIDGE_PID" >> "$PIDS_FILE"
log "Bridge started with PID ${BRIDGE_PID}"

# Wait for bridge to become healthy
RETRIES=0
while [[ $RETRIES -lt 15 ]]; do
    if curl -sf "http://127.0.0.1:${BRIDGE_PORT}/health" >/dev/null 2>&1; then
        log "Bridge server is ready"
        break
    fi

    if ! kill -0 "$BRIDGE_PID" 2>/dev/null; then
        log_error "Bridge process died during startup"
        cat /tmp/bridge.log >&2
        report_status "error" "Bridge server failed to start"
        exit 1
    fi

    RETRIES=$((RETRIES + 1))
    sleep 1
done

if [[ $RETRIES -ge 15 ]]; then
    log_error "Bridge did not become ready in time"
    cat /tmp/bridge.log >&2
    report_status "error" "Bridge startup timeout"
    exit 1
fi

# ---------------------------------------------------------------------------
# Step 6: Send initial prompt to agent
# ---------------------------------------------------------------------------
log "Sending initial prompt to agent"
report_status "running" "Agent is ready, sending initial prompt..."

INITIAL_RESPONSE=$(curl -sf -X POST \
    "http://127.0.0.1:${BRIDGE_PORT}/message" \
    -H "Content-Type: application/json" \
    -d "$(jq -n --arg p "$AGENT_PROMPT" '{message: $p}')" \
    2>&1) || true

if [[ -n "$INITIAL_RESPONSE" ]]; then
    log "Initial prompt sent, agent is working"
    report_status "running" "Agent is actively working on the task"
else
    log "Initial prompt sent (no immediate response)"
fi

# ---------------------------------------------------------------------------
# Step 7: Wait for child processes
# ---------------------------------------------------------------------------
log "Agent pod is running. Waiting for processes..."

# Monitor child processes — exit if either dies
while true; do
    if ! kill -0 "$SMOLAGENT_PID" 2>/dev/null; then
        log_error "smolagent process exited"
        SMOLAGENT_EXIT=$(wait "$SMOLAGENT_PID" 2>/dev/null; echo $?)
        log "smolagent exit code: ${SMOLAGENT_EXIT}"
        break
    fi

    if ! kill -0 "$BRIDGE_PID" 2>/dev/null; then
        log_error "Bridge process exited"
        BRIDGE_EXIT=$(wait "$BRIDGE_PID" 2>/dev/null; echo $?)
        log "Bridge exit code: ${BRIDGE_EXIT}"
        break
    fi

    sleep 5
done

log "Agent pod main loop ended"

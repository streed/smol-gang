#!/bin/bash
# Cleanup script: commit, push, and report final status
# Usage: cleanup.sh <workspace> <branch_name> <gateway_url> <workstream_id> <gateway_token> <exit_code>
set -e

WORKSPACE="${1:-/workspace/repo}"
BRANCH_NAME="${2:-${BRANCH_NAME:-}}"
GATEWAY_URL="${3:-${GATEWAY_URL:-}}"
WORKSTREAM_ID="${4:-${WORKSTREAM_ID:-}}"
GATEWAY_TOKEN="${5:-${GATEWAY_TOKEN:-}}"
EXIT_CODE="${6:-0}"

echo "=== Starting cleanup (exit_code=${EXIT_CODE}) ==="

cd "$WORKSPACE"

# Stage all changes
git add -A

# Commit if there are changes
if ! git diff --cached --quiet; then
    git commit -m "chore: final agent changes

Automated commit by smol-cluster agent"
    echo "Changes committed"
else
    echo "No changes to commit"
fi

# Push branch
if [ -n "$BRANCH_NAME" ]; then
    echo "Pushing branch: $BRANCH_NAME"
    git push -u origin "$BRANCH_NAME" || echo "Warning: push failed"
fi

# Determine final status based on exit code
if [ "$EXIT_CODE" -eq 0 ]; then
    FINAL_STATUS="completed"
else
    FINAL_STATUS="failed"
fi

# Report status to gateway
if [ -n "$GATEWAY_URL" ] && [ -n "$WORKSTREAM_ID" ]; then
    AUTH_HEADER=""
    if [ -n "$GATEWAY_TOKEN" ]; then
        AUTH_HEADER="-H \"Authorization: Bearer $GATEWAY_TOKEN\""
    fi
    curl -s -X POST \
        "$GATEWAY_URL/api/v1/internal/workstreams/$WORKSTREAM_ID/status" \
        -H "Content-Type: application/json" \
        ${GATEWAY_TOKEN:+-H "Authorization: Bearer $GATEWAY_TOKEN"} \
        -d "{\"status\":\"$FINAL_STATUS\"}" \
        || echo "Warning: failed to report status"
fi

echo "=== Cleanup complete (status=${FINAL_STATUS}) ==="

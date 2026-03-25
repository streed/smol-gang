#!/bin/bash
# Cleanup script: commit, push, and report final status
set -e

WORKSPACE="/workspace/repo"
BRANCH_NAME="${BRANCH_NAME:-}"
GATEWAY_URL="${GATEWAY_URL:-}"
WORKSTREAM_ID="${WORKSTREAM_ID:-}"

echo "=== Starting cleanup ==="

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

# Report completion to gateway
if [ -n "$GATEWAY_URL" ] && [ -n "$WORKSTREAM_ID" ]; then
    curl -s -X POST \
        "$GATEWAY_URL/api/v1/internal/workstreams/$WORKSTREAM_ID/status" \
        -H "Content-Type: application/json" \
        -d '{"status":"completed"}' \
        || echo "Warning: failed to report status"
fi

echo "=== Cleanup complete ==="

#!/bin/bash
# App runner entrypoint - runs the user's application in isolation from the agent.
# Waits for the agent to clone the repo and complete setup before starting.
set -e

WORKSPACE="/workspace/repo"
SETUP_DONE_FLAG="/workspace/.setup-done"
APP_COMMANDS="${APP_COMMANDS:-}"

echo "=== App Runner Starting ==="
echo "Waiting for agent to complete repo setup..."

# Wait for the agent to signal that setup is done
while [ ! -f "$SETUP_DONE_FLAG" ]; do
    sleep 2
done

echo "Setup complete, starting application..."

cd "$WORKSPACE"

# Run app commands from .smol-cluster.yaml or env override
if [ -n "$APP_COMMANDS" ]; then
    echo "Running app commands: $APP_COMMANDS"
    IFS=';' read -ra CMDS <<< "$APP_COMMANDS"
    for cmd in "${CMDS[@]}"; do
        cmd=$(echo "$cmd" | xargs)
        if [ -n "$cmd" ]; then
            echo "Running: $cmd"
            eval "$cmd" &
        fi
    done
fi

# If no explicit app commands, try to detect and run common dev servers
if [ -z "$APP_COMMANDS" ]; then
    if [ -f "package.json" ]; then
        echo "Detected Node.js project"
        if grep -q '"dev"' package.json 2>/dev/null; then
            npm run dev &
        elif grep -q '"start"' package.json 2>/dev/null; then
            npm start &
        fi
    elif [ -f "docker-compose.yml" ] || [ -f "docker-compose.yaml" ] || [ -f "compose.yml" ]; then
        echo "Detected Docker Compose project"
        docker compose up &
    elif [ -f "manage.py" ]; then
        echo "Detected Django project"
        python3 manage.py runserver 0.0.0.0:8000 &
    elif [ -f "requirements.txt" ] && [ -f "app.py" ]; then
        echo "Detected Flask/Python project"
        python3 app.py &
    elif [ -f "Makefile" ] && grep -q '^dev:' Makefile 2>/dev/null; then
        echo "Detected Makefile dev target"
        make dev &
    else
        echo "No known project type detected, idling (agent can start services)"
    fi
fi

# Keep the container alive, watching for file changes
echo "App runner active. Monitoring workspace..."
while true; do
    # If the agent signals a restart, re-run app commands
    if [ -f "/workspace/.restart-app" ]; then
        rm -f "/workspace/.restart-app"
        echo "Restart signal received, restarting app..."
        # Kill child processes and restart
        kill $(jobs -p) 2>/dev/null || true
        exec "$0"
    fi
    sleep 5
done

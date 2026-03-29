#!/bin/bash
# Setup script for agent workspace
# Reads .smol-gang.yaml and runs setup commands

set -e

WORKSPACE="/workspace/repo"

echo "=== Running setup ==="

# Check for .smol-gang.yaml
if [ -f "$WORKSPACE/.smol-gang.yaml" ]; then
    echo "Found .smol-gang.yaml"

    # Extract setup_commands entries using grep/sed (no Python needed)
    # Supports simple YAML list format:
    #   setup_commands:
    #     - npm install
    #     - npm run build
    sed -n '/^setup_commands:/,/^[^ ]/{ /^  *- /p }' "$WORKSPACE/.smol-gang.yaml" \
        | sed 's/^  *- //' \
        | while IFS= read -r cmd; do
        echo "Running: $cmd"
        (cd "$WORKSPACE" && eval "$cmd") || echo "Warning: command failed: $cmd"
    done
fi

# Run any commands passed via SETUP_COMMANDS env var (semicolon-separated)
if [ -n "$SETUP_COMMANDS" ]; then
    echo "Running setup commands from environment"
    IFS=';' read -ra CMDS <<< "$SETUP_COMMANDS"
    for cmd in "${CMDS[@]}"; do
        cmd=$(echo "$cmd" | xargs)  # trim whitespace
        if [ -n "$cmd" ]; then
            echo "Running: $cmd"
            (cd "$WORKSPACE" && eval "$cmd") || echo "Warning: command failed: $cmd"
        fi
    done
fi

echo "=== Setup complete ==="

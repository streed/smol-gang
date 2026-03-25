#!/bin/bash
# Setup script for agent workspace
# Reads .smol-cluster.yaml and runs setup commands

set -e

WORKSPACE="/workspace/repo"

echo "=== Running setup ==="

# Check for .smol-cluster.yaml
if [ -f "$WORKSPACE/.smol-cluster.yaml" ]; then
    echo "Found .smol-cluster.yaml"

    # Extract setup commands using python (available in the container)
    python3 -c "
import yaml
import sys

try:
    with open('$WORKSPACE/.smol-cluster.yaml') as f:
        config = yaml.safe_load(f)

    commands = config.get('setup_commands', [])
    for cmd in commands:
        print(cmd)
except Exception as e:
    print(f'Warning: failed to parse .smol-cluster.yaml: {e}', file=sys.stderr)
" | while IFS= read -r cmd; do
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

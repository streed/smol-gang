# Minimal agent container - only smolagent + bridge, no app runtimes.
# The agent shares a workspace volume with a separate "app" container
# so that agent crashes don't take down the running application.
FROM python:3.12-slim

# Only install what the agent needs: git for repo ops, curl for gateway callbacks
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    jq \
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

# Install smolagents + bridge deps only
RUN pip install --no-cache-dir smolagents litellm pyyaml

# Create directories
RUN mkdir -p /workspace /app

COPY scripts/ /app/scripts/
RUN chmod +x /app/scripts/*.sh

WORKDIR /workspace

# The entrypoint handles: clone repo, create branch, start agent + bridge
ENTRYPOINT ["/app/scripts/entrypoint.sh"]

FROM python:3.12-slim

# Install system deps
RUN apt-get update && apt-get install -y \
    git \
    curl \
    jq \
    openssh-client \
    nodejs \
    npm \
    && rm -rf /var/lib/apt/lists/*

# Install smolagents
RUN pip install --no-cache-dir smolagents litellm

# Create workspace directory
RUN mkdir -p /workspace /app

COPY scripts/ /app/scripts/
RUN chmod +x /app/scripts/*.sh

WORKDIR /workspace

# The entrypoint handles: clone repo, create branch, start agent
ENTRYPOINT ["/app/scripts/entrypoint.sh"]

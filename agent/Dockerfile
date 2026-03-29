FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    jq \
    ca-certificates \
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

# Install smol-agent
RUN git clone --depth=1 https://github.com/streed/smol-agent.git /opt/smol-agent \
    && cd /opt/smol-agent \
    && npm install --production \
    && chmod +x src/index.js \
    && ln -sf /opt/smol-agent/src/index.js /usr/local/bin/smol-agent

RUN mkdir -p /workspace /app

COPY scripts/ /app/scripts/
RUN chmod +x /app/scripts/*.sh

WORKDIR /workspace

ENTRYPOINT ["/app/scripts/entrypoint.sh"]

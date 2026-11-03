# Node.js LTS mit corepack (pnpm, yarn). Seit Node 25 gehört corepack nicht mehr zur Distribution.
# Mit BASE=boxai-kotlin-<agent> entsteht daraus das Fullstack-Image.
ARG BASE=boxai-base-claude:latest
FROM ${BASE}
ARG NODE_VERSION=26.13.0
USER root
RUN set -eu; \
    arch="$(uname -m)"; \
    case "$arch" in x86_64) node=x64 ;; aarch64) node=arm64 ;; *) echo "Architektur $arch nicht unterstützt" >&2; exit 1 ;; esac; \
    mkdir -p /opt/node; \
    curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node}.tar.gz" | tar -xz --strip-components=1 -C /opt/node; \
    for tool in node npm npx; do ln -sf "/opt/node/bin/$tool" "/usr/local/bin/$tool"; done; \
    /opt/node/bin/npm install -g corepack; \
    ln -sf /opt/node/bin/corepack /usr/local/bin/corepack
USER agent

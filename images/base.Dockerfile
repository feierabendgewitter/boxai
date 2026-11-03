# Default-Tools aller mitgelieferten Images. Git, gh, jq und curl bringt das Template mit.
ARG BASE=docker/sandbox-templates:claude-code-docker
FROM ${BASE}
ARG AGENT=claude
ARG GLAB_VERSION=1.128.0
LABEL boxai.agent="${AGENT}"
USER root
RUN set -eu; \
    apt-get update; \
    apt-get install -y --no-install-recommends git jq curl unzip ca-certificates; \
    arch="$(dpkg --print-architecture)"; \
    curl -fsSL -o /tmp/glab.deb "https://gitlab.com/gitlab-org/cli/-/releases/v${GLAB_VERSION}/downloads/glab_${GLAB_VERSION}_linux_${arch}.deb"; \
    apt-get install -y /tmp/glab.deb; \
    curl -fsSL -o /tmp/aws.zip "https://awscli.amazonaws.com/awscli-exe-linux-$(uname -m).zip"; \
    unzip -q /tmp/aws.zip -d /tmp; \
    /tmp/aws/install; \
    rm -rf /tmp/aws /tmp/aws.zip /tmp/glab.deb /var/lib/apt/lists/*; \
    command -v gh
USER agent

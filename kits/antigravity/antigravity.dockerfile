FROM docker/sandbox-templates:shell-docker
USER agent
RUN curl -fsSL https://antigravity.google/cli/install.sh | bash
ENV PATH=/home/agent/.local/bin:$PATH
ENTRYPOINT ["agy"]
CMD []

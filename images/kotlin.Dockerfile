# Temurin JDK 21 und Maven; das Template bringt ein neueres OpenJDK ohne Maven. Gradle kommt über den Wrapper der Repos.
ARG BASE=boxai-base-claude:latest
FROM ${BASE}
ARG MAVEN_VERSION=3.9.16
USER root
RUN set -eu; \
    arch="$(uname -m)"; \
    case "$arch" in x86_64) jdk=x64 ;; aarch64) jdk=aarch64 ;; *) echo "Architektur $arch nicht unterstützt" >&2; exit 1 ;; esac; \
    mkdir -p /opt/jdk /opt/maven; \
    curl -fsSL "https://api.adoptium.net/v3/binary/latest/21/ga/linux/${jdk}/jdk/hotspot/normal/eclipse" | tar -xz --strip-components=1 -C /opt/jdk; \
    curl -fsSL "https://repo.maven.apache.org/maven2/org/apache/maven/apache-maven/${MAVEN_VERSION}/apache-maven-${MAVEN_VERSION}-bin.tar.gz" | tar -xz --strip-components=1 -C /opt/maven; \
    for tool in java javac jar; do ln -sf "/opt/jdk/bin/$tool" "/usr/local/bin/$tool"; done; \
    ln -sf /opt/maven/bin/mvn /usr/local/bin/mvn
ENV JAVA_HOME=/opt/jdk
USER agent

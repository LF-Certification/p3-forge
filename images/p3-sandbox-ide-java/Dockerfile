# syntax=docker/dockerfile:1.7

FROM eclipse-temurin:21.0.8_9-jdk@sha256:7d1d666ddafac14da0ded6b4b076becf76cf88b31f9d7953a76555cc82f86511 AS java

FROM ghcr.io/lf-certification/p3-sandbox-ide:v1.3.0@sha256:0cf8f45ca811cb45aa4d828b0ff95e5cf7bd9f8541256f1f7436382e542b8c95

USER root
COPY --from=java /opt/java/openjdk /opt/java/openjdk
ENV JAVA_HOME=/opt/java/openjdk
ENV PATH="${JAVA_HOME}/bin:${PATH}"

# code-server 4.22.0 embeds VS Code 1.87.0. redhat.java 1.55.0
# declares engines.vscode ^1.77.0, so the pinned host is compatible.
USER 1000:1000
RUN curl --fail --location --silent --show-error \
      https://open-vsx.org/api/redhat/java/1.55.0/file/redhat.java-1.55.0.vsix \
      --output /tmp/redhat.java.vsix \
    && echo "011639c3ee347b9591895bfc77d8cf28f836c053d0a49ae4a83efe9dc473a603  /tmp/redhat.java.vsix" \
      | sha256sum --check --strict \
    && code-server --install-extension /tmp/redhat.java.vsix \
    && rm /tmp/redhat.java.vsix

# The sandbox operator mounts the target user's home at shared/workspace, so
# jdt.ls resolves the prepared Maven repository directly from the VM.
COPY --chown=1000:1000 maven-settings.xml /home/coder/.m2/settings.xml

# Keep the base image's terminal and watcher defaults at Machine scope. Java
# policy lives at User scope so code-server merges both settings layers.
COPY --chown=1000:1000 settings.json /home/coder/.local/share/code-server/User/settings.json

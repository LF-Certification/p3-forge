# syntax=docker/dockerfile:1.7

FROM codercom/code-server:4.132.0 AS base

COPY --chown=1000:1000 settings.json /home/coder/.local/share/code-server/Machine/settings.json
COPY code-server.yaml /etc/code-server.yaml
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY remote-shell.sh /usr/local/bin/p3-ide-remote-shell

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

FROM eclipse-temurin:21.0.8_9-jdk@sha256:7d1d666ddafac14da0ded6b4b076becf76cf88b31f9d7953a76555cc82f86511 AS java-runtime

FROM base AS java

USER root
COPY --from=java-runtime /opt/java/openjdk /opt/java/openjdk
ENV JAVA_HOME=/opt/java/openjdk
ENV PATH="${JAVA_HOME}/bin:${PATH}"

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
COPY --chown=1000:1000 variants/java/maven-settings.xml /home/coder/.m2/settings.xml

# Keep the base image's terminal and watcher defaults at Machine scope. Java
# policy lives at User scope so code-server merges both settings layers.
COPY --chown=1000:1000 variants/java/settings.json /home/coder/.local/share/code-server/User/settings.json

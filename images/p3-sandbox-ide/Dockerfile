FROM codercom/code-server:4.22.0

COPY settings.json /root/.local/share/code-server/Machine/settings.json
COPY code-server.yaml /etc/code-server.yaml
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
USER root
RUN chmod +x /usr/local/bin/entrypoint.sh

# Expose code-server port
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

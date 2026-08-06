FROM codercom/code-server:4.22.0

COPY --chown=1000:1000 settings.json /home/coder/.local/share/code-server/Machine/settings.json
COPY code-server.yaml /etc/code-server.yaml
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY remote-shell.sh /usr/local/bin/p3-ide-remote-shell

# Expose code-server port
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

FROM codercom/code-server:4.22.0

# Copy entrypoint script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
USER root
RUN chmod +x /usr/local/bin/entrypoint.sh

# Expose code-server port
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

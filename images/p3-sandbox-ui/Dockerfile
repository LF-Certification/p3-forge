FROM nginx:alpine

# Copy HTML template and entrypoint script, ensure script has proper permissions
COPY scripts/index.html /usr/share/nginx/html/index.html
COPY scripts/ui-entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 80

ENTRYPOINT ["/entrypoint.sh"]

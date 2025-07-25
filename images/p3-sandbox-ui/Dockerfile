FROM caddy:2-alpine
LABEL org.opencontainers.image.source="https://github.com/lf-certification/lab-images"
RUN mkdir -p /usr/share/caddy/dist /usr/share/caddy/config
COPY lab-ui/dist/ /usr/share/caddy/dist/
COPY dev/config.json /usr/share/caddy/config.json
COPY dev/Caddyfile /etc/caddy/Caddyfile
EXPOSE 80
WORKDIR /usr/share/caddy

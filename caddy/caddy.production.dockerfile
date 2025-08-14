FROM caddy:2.4.6-alpine

# Copy in your production Caddyfile
COPY Caddyfile.production /etc/caddy/Caddyfile

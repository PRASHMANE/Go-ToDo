# Frontend Phase 5 — Docker Image with Nginx & Compose Integration

**Goal:** `docker compose up` also serves the React app at http://localhost:3000.
Nginx serves static files **and** proxies `/api` to the gateway, so the browser
sees one origin (no CORS in production).
**Branch:** `phase/f5-frontend-docker` · **Tag:** `f5`

## Concepts

- `npm run build` turns your React code into **static files** (`dist/`: one
  HTML file, hashed JS/CSS). No Node.js is needed at runtime.
- **Nginx** is a fast web server. Two jobs here:
  1. Serve `dist/`, falling back to `index.html` for unknown paths (**SPA
     fallback**), so `/login` works when you refresh on it.
  2. Reverse-proxy `/api/*` to `api-gateway:8080` (same idea as our Go gateway, one level up).
- **Cache headers:** files in `/assets/` have a content hash in their name
  (`index-a1b2c3.js`), so they can be cached "forever". `index.html` must
  **never** be cached, or users get stuck on old versions.
- `VITE_*` env vars are baked in **at build time**, not runtime. Hence the build `ARG`.

## Step 1 — `frontend/nginx.conf`

```nginx
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    # API → gateway (service name on the compose network)
    location /api/ {
        proxy_pass http://api-gateway:8080;
        proxy_http_version 1.1;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
    }

    # Hashed build assets: cache for a year
    location /assets/ {
        expires 1y;
        try_files $uri =404;
    }

    # SPA fallback: unknown paths → index.html (React Router handles them)
    location / {
        expires -1;                       # Cache-Control: no-cache
        try_files $uri $uri/ /index.html;
    }

    # Basic security headers.
    # ⚠️ nginx gotcha: if a location{} block has ANY add_header of its own, it
    # stops inheriting these. That's why the locations above use `expires`
    # (which is not add_header) for caching.
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript image/svg+xml;
}
```

## Step 2 — `frontend/Dockerfile`

```dockerfile
# syntax=docker/dockerfile:1

# ---------- build ----------
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci                                # exact versions from the lockfile
COPY . .
ARG VITE_API_URL=/api/v1
ENV VITE_API_URL=$VITE_API_URL
RUN npm run build

# ---------- runtime ----------
FROM nginx:1.29-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
# 127.0.0.1, not localhost: in alpine, localhost resolves to IPv6 ::1 but nginx listens on IPv4
HEALTHCHECK --interval=10s --timeout=3s CMD wget -qO- http://127.0.0.1/ > /dev/null || exit 1
```

`npm ci` vs `npm install`: `ci` installs exactly what `package-lock.json`
says and fails if it's out of sync. Always use it in Docker/CI.

## Step 3 — `frontend/.dockerignore`

```
node_modules
dist
.env
.env.*
!.env.example
```

## Step 4 — Add to `docker-compose.yml`

```yaml
  frontend:
    build:
      context: ./frontend
      args:
        VITE_API_URL: /api/v1
    ports:
      - "3000:80"
    depends_on:
      api-gateway: { condition: service_healthy }
    restart: unless-stopped
```

And in the **api-gateway** environment, trust the Nginx container so
`c.ClientIP()` (rate limiting) uses the real client IP from `X-Forwarded-For`:

```yaml
      TRUSTED_PROXIES: 172.16.0.0/12      # Docker's default private address range
```

## Step 5 — Makefile

```makefile
open:            ## Open the app
	open http://localhost:3000
```

## Verify

```bash
make up && make ps          # frontend "healthy"
make open
```

- Register, log in, use the app at **:3000**. Everything goes through Nginx → gateway → services.
- Go to http://localhost:3000/login and refresh → still works (SPA fallback).
- DevTools → Network: requests go to `localhost:3000/api/v1/...`, with no CORS preflight `OPTIONS` requests (same origin).
- `curl -sI localhost:3000/assets/<some-file>.js | grep -i cache` → `max-age=31536000`.
- `curl -sI localhost:3000/ | grep -iE 'cache|x-frame'` → `no-cache` and `DENY`.
- `docker images | grep frontend`: compare the size with `node:22-alpine`.

## Exercises

1. Why would `proxy_pass http://api-gateway:8080/;` (with a trailing slash) break the API? Try it.
2. Remove `try_files ... /index.html` and refresh on `/login`. What happens?
3. Add a `Content-Security-Policy` header that only allows scripts from `'self'`.

## Commit

```bash
git add -A
git commit -m "build(frontend): nginx image and compose integration (F5)"
```

**Next:** [Backend Phase 7 — Testing](../backend/phase-7-testing.md) →
[Backend Phase 8 — Events](../backend/phase-8-events-activity-service.md) →
[Frontend Phase 6](phase-6-testing-activity-polish.md)

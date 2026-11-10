#!/usr/bin/env bash
# Lives at: /usr/local/bin/go-todo-deploy
# Build + migrate + restart. Run on the VM:  sudo go-todo-deploy
set -euo pipefail

APP_DIR=/opt/go-todo
APP_USER=gotodo
ENV_DIR=/etc/go-todo
export PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin

as_app() { sudo -u "$APP_USER" -H env PATH="$PATH" "$@"; }

cd "$APP_DIR"

echo "==> 1/5 pull latest code"
as_app git pull --ff-only

echo "==> 2/5 build Go binaries → bin/"
as_app make release

echo "==> 3/5 build frontend → frontend/dist"
as_app sh -c 'cd frontend && npm ci --no-audit --no-fund && npm run build'

echo "==> 4/5 make sure Postgres + NATS are up, then run migrations"
docker compose -f "$ENV_DIR/docker-compose.infra.yml" --env-file "$ENV_DIR/infra.env" up -d --wait
for svc in auth todo activity; do
  url=$(grep '^DATABASE_URL=' "$ENV_DIR/$svc.env" | cut -d= -f2-)
  docker run --rm --network host \
    -v "$APP_DIR/services/$svc-service/migrations:/migrations:ro" \
    migrate/migrate:v4.20.1 -path=/migrations -database "$url" up
done

echo "==> 5/5 restart services (supervisord)"
supervisorctl reread
supervisorctl update
supervisorctl restart 'go-todo:*'
sleep 4
supervisorctl status 'go-todo:*'

echo "==> health"
curl -fsS http://127.0.0.1:8080/healthz && echo
for p in 8081 8082 8083; do curl -fsS "http://127.0.0.1:$p/readyz" && echo; done

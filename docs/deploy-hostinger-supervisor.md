# Deploy to a Hostinger VPS with supervisord

Run Go-ToDo on one Ubuntu VPS. **supervisord** keeps the 4 Go services running,
**nginx** serves the React app and forwards `/api` to the gateway, and **Docker**
runs only Postgres + NATS.

```
Browser ──https──▶ nginx :443 ─┬─ /          → /opt/go-todo/frontend/dist   (static files)
                               └─ /api/*     → 127.0.0.1:8080  api-gateway ─┬─ /api/v1/auth/*     → :8081 auth-service
                                                                            ├─ /api/v1/todos/*    → :8082 todo-service
                                                                            └─ /api/v1/activity   → :8083 activity-service
supervisord (started by systemd at boot) keeps those 4 Go binaries running.
Docker: postgres 127.0.0.1:5432, nats 127.0.0.1:4222   (never reachable from the internet)
```

## How supervisor works (the mental model)

| Piece | Role |
|-------|------|
| `systemd` | Starts `supervisord` when the VM boots. |
| `supervisord` | The daemon. Reads `/etc/supervisor/conf.d/*.conf`, starts every `[program:x]`, restarts it if it crashes, writes its logs. |
| `/etc/supervisor/conf.d/go-todo.conf` | **Where your start commands live.** One `[program:…]` block per service with `command=…`. |
| `supervisorctl` | The remote control: `status`, `start`, `stop`, `restart`, `tail`. It doesn't run your commands itself; it asks supervisord to. |

Each program's `command=` is:

```
/opt/go-todo/deployments/prod/run-with-env.sh /etc/go-todo/todo.env /opt/go-todo/bin/todo-service
```

`run-with-env.sh` loads the secrets file and then `exec`s the binary. Because
of `exec`, supervisord's stop signal (SIGTERM) reaches the Go program directly,
and our graceful shutdown from Phase B1 runs.

## Files in the repo (already written and tested)

| File | Goes to / used by |
|------|-------------------|
| `deployments/prod/supervisor/go-todo.conf` | `/etc/supervisor/conf.d/go-todo.conf` |
| `deployments/prod/nginx/go-todo.conf` | `/etc/nginx/sites-available/go-todo` |
| `deployments/prod/env/*.env.example` | copied to `/etc/go-todo/*.env` and filled with real secrets |
| `deployments/prod/docker-compose.infra.yml` | Postgres + NATS only, bound to `127.0.0.1` |
| `deployments/prod/postgres-init.sh` | creates the 3 databases/users on first start |
| `deployments/prod/run-with-env.sh` | the wrapper in every `command=` |
| `deployments/prod/deploy.sh` | pull → build → migrate → `supervisorctl restart` |
| `make release` | builds the 4 binaries into `bin/` |

---

## Step 0 — Before you start

- Push the repo to GitHub from your Mac: `git push origin main --tags`.
- In Hostinger hPanel → VPS: use **Ubuntu 24.04**, note the IP, and SSH in:
  `ssh root@YOUR_VPS_IP`
- (Optional) Point a domain's **A record** at the VPS IP (needed for HTTPS).

> Run every command below on the VPS. Copy them one block at a time.
> There are no `# comments` after commands, so zsh/bash won't trip over them.

## Step 1 — Install packages

```bash
apt update && apt upgrade -y
```
```bash
apt install -y git make curl ca-certificates nginx supervisor ufw
```

Docker (only for Postgres + NATS):

```bash
curl -fsSL https://get.docker.com | sh
```

Go 1.27.3 (Ubuntu's apt version is too old):

```bash
curl -fsSL https://go.dev/dl/go1.27.3.linux-amd64.tar.gz -o /tmp/go.tgz
rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
```
```bash
. /etc/profile.d/go.sh && go version
```

Node 22 (to build the React app):

```bash
curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
apt install -y nodejs
```
```bash
node -v && npm -v
```

Check supervisor is running:

```bash
systemctl enable --now supervisor
systemctl status supervisor --no-pager
```

## Step 2 — Firewall

Allow only SSH and web traffic:

```bash
ufw allow OpenSSH
ufw allow 'Nginx Full'
ufw --force enable
ufw status
```

Also open 80/443 in **hPanel → VPS → Firewall** if you use Hostinger's firewall.
(Postgres/NATS are bound to 127.0.0.1, so they're private even though Docker bypasses ufw.)

## Step 3 — App user and code

A dedicated, non-root user runs the services:

```bash
adduser --system --group --home /home/gotodo gotodo
mkdir -p /opt/go-todo /var/log/go-todo
chown gotodo:gotodo /opt/go-todo /var/log/go-todo
```

Clone (for a private repo, use a GitHub deploy key or a token URL):

```bash
sudo -u gotodo git clone https://github.com/PRASHMANE/Go-ToDo.git /opt/go-todo
```

## Step 4 — Secrets in `/etc/go-todo`

```bash
mkdir -p /etc/go-todo
cd /opt/go-todo/deployments/prod/env
for f in infra auth todo activity gateway; do cp $f.env.example /etc/go-todo/$f.env; done
```

Generate values (run each once and copy the output):

```bash
openssl rand -hex 24
```
```bash
openssl rand -hex 48
```

Edit the files:

```bash
nano /etc/go-todo/infra.env
```

- `infra.env`: 4 passwords (`openssl rand -hex 24` each).
- `auth.env` / `todo.env` / `activity.env`: put the **same** DB password from
  `infra.env` into each `DATABASE_URL` (replace `AUTH_DB_PASSWORD`, etc.).
- `auth.env` and `gateway.env`: the **same** `JWT_SECRET` (`openssl rand -hex 48`).
- `gateway.env`: `CORS_ORIGINS=https://your-domain` (or `http://YOUR_VPS_IP` for a first test).

Lock them down (readable by root and the `gotodo` group only):

```bash
chown root:gotodo /etc/go-todo/*.env
chmod 640 /etc/go-todo/*.env
```

## Step 5 — Start Postgres + NATS

```bash
cd /opt/go-todo
docker compose --env-file /etc/go-todo/infra.env -f deployments/prod/docker-compose.infra.yml up -d --wait
```
```bash
docker compose -f deployments/prod/docker-compose.infra.yml ps
```

`postgres-init.sh` creates `auth_db`, `todo_db` and `activity_db` with your
passwords on this first start. (Changing passwords later means changing them in
Postgres too, or wiping the volume.)

## Step 6 — Install the supervisor config

```bash
cp /opt/go-todo/deployments/prod/supervisor/go-todo.conf /etc/supervisor/conf.d/go-todo.conf
```

## Step 7 — First deploy (build + migrate + start)

```bash
/opt/go-todo/deployments/prod/deploy.sh
```

It runs: `git pull` → `make release` → frontend `npm ci && npm run build` →
starts infra + runs migrations → `supervisorctl reread / update / restart go-todo:*`
→ prints status and health checks. You should see:

```
go-todo:activity-service   RUNNING   pid 1234, uptime 0:00:04
go-todo:api-gateway        RUNNING   ...
go-todo:auth-service       RUNNING   ...
go-todo:todo-service       RUNNING   ...
{"status":"ok"}
{"checks":{"db":"ok"},"status":"ready"} ...
```

## Step 8 — nginx (+ HTTPS)

Set your domain (or the IP) in the file first:

```bash
nano /opt/go-todo/deployments/prod/nginx/go-todo.conf
```

Then enable it:

```bash
cp /opt/go-todo/deployments/prod/nginx/go-todo.conf /etc/nginx/sites-available/go-todo
ln -sf /etc/nginx/sites-available/go-todo /etc/nginx/sites-enabled/go-todo
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl reload nginx
```

Open `http://YOUR_DOMAIN_OR_IP`, register and add todos. 🎉

HTTPS (needs the domain's A record pointing at the VPS):

```bash
apt install -y certbot python3-certbot-nginx
certbot --nginx -d todo.example.com
```

Then make sure `CORS_ORIGINS` in `gateway.env` uses `https://…`, and run
`supervisorctl restart go-todo:api-gateway`.

---

## Daily operations

| Task | Command |
|------|---------|
| Status of all services | `supervisorctl status` |
| Restart everything | `supervisorctl restart 'go-todo:*'` |
| Restart one | `supervisorctl restart go-todo:todo-service` |
| Live logs of one | `supervisorctl tail -f go-todo:todo-service` |
| Log files | `ls /var/log/go-todo/` |
| After editing `go-todo.conf` | `supervisorctl reread && supervisorctl update` |
| After editing an `.env` file | `supervisorctl restart go-todo:<that-service>` |
| Deploy new code (after `git push` from your Mac) | `/opt/go-todo/deployments/prod/deploy.sh` |
| DB shell | `docker exec -it go-todo-infra-postgres-1 psql -U postgres` |

Everything comes back by itself after a reboot: systemd starts Docker and
supervisord, Docker restarts Postgres/NATS (`restart: unless-stopped`), and
supervisord starts the Go services (`autostart=true`; `startretries=10` covers the
few seconds Postgres needs).

## Troubleshooting

| Symptom | Look at / fix |
|---------|---------------|
| `FATAL` or `BACKOFF` in `supervisorctl status` | `supervisorctl tail go-todo:<name>`: usually a wrong value in `/etc/go-todo/<name>.env` (e.g. `required environment variable JWT_SECRET is not set`, `password authentication failed`). |
| `spawn error` | Binary missing (`ls /opt/go-todo/bin`, re-run `deploy.sh`) or the wrapper isn't executable (`chmod +x deployments/prod/*.sh`). |
| `permission denied` reading `.env` | `chown root:gotodo /etc/go-todo/*.env && chmod 640 /etc/go-todo/*.env` |
| `502 Bad Gateway` from nginx | Gateway not running: `supervisorctl status go-todo:api-gateway`. |
| `{"code":"bad_gateway"}` JSON | Gateway is up, but a service behind it isn't. Check status of the others. |
| Login works but every call is 401 | `JWT_SECRET` differs between `auth.env` and `gateway.env`. |
| `address already in use` | Something else on 8080–8083: `ss -ltnp \| grep 808`. |
| Old frontend after deploy | Hard refresh (Ctrl+Shift+R). `index.html` is `no-cache`, so it's usually the browser. |

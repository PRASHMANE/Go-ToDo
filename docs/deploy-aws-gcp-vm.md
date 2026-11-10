# Deploy Go-ToDo to a Cloud VM (AWS or GCP) — from scratch

This guide starts from an **empty cloud account** and ends with Go-ToDo live on
the internet. Along the way you build:

1. a private network (**VPC**) with a public subnet,
2. a **firewall** (AWS Security Group / GCP firewall rules, plus `ufw` on the VM),
3. an **Ubuntu 24.04 VM** with a static public IP,
4. **supervisord**, which starts and keeps the 4 Go services running,
5. **nginx**, which serves the React app and forwards `/api` to the gateway,
6. **HTTPS** with Let's Encrypt.

Part A is for **AWS**, Part B is for **GCP**. Do one of them, then continue with
**Part C (the VM)**, which is identical on both clouds.

```
                        ┌──────────────────────── VPC 10.0.0.0/16 ────────────────────────┐
                        │  public subnet 10.0.1.0/24                                       │
Internet ──▶ Firewall ──┼─▶ VM (Ubuntu 24.04, static public IP)                            │
 (80, 443 open,         │     ufw: 22, 80, 443                                             │
  22 only from your IP) │     nginx :80/:443 ─┬─ /       → /opt/go-todo/frontend/dist       │
                        │                     └─ /api/*  → 127.0.0.1:8080 api-gateway      │
                        │     supervisord ─▶ api-gateway :8080 ─┬─▶ auth-service     :8081  │
                        │                                       ├─▶ todo-service     :8082  │
                        │                                       └─▶ activity-service :8083  │
                        │     docker ─▶ postgres 127.0.0.1:5432, nats 127.0.0.1:4222       │
                        └──────────────────────────────────────────────────────────────────┘
```

Only nginx is reachable from the internet. The gateway, the 3 services,
Postgres and NATS listen on the VM but are blocked from outside.

> **Copy rule:** none of the command blocks contain `# comments`, so they're
> safe to paste into zsh on your Mac. Explanations are always outside the blocks.

> **Cost:** a `t3.small` (AWS) or `e2-small` (GCP) costs roughly US$15–20/month,
> plus a few dollars for disk and the static IP. Part D shows how to delete
> everything when you're done.

---

## Before you start (on your Mac)

Push the code so the VM can clone it:

```bash
cd ~/Documents/Go_ToDo_Project
git push origin main --tags
```

You'll need your public IP to allow SSH only from you:

```bash
export MY_IP=$(curl -s https://checkip.amazonaws.com)
echo $MY_IP
```

Run all the cloud commands **in the same terminal window**, because later
commands reuse the variables (`$VPC_ID`, `$MY_IP`, …) set by earlier ones.

---

# Part A — AWS

## A1. AWS account and CLI

1. Create an AWS account at https://aws.amazon.com and **turn on MFA** for the root user.
2. In **IAM**, create a user for yourself (e.g. `prashanth-admin`) with the
   `AdministratorAccess` policy, then create an **access key** for it
   (use case: "Command Line Interface").
3. Configure the CLI (already installed: `aws --version`):

```bash
aws configure
```

Enter the access key, the secret, region **`ap-south-1`** (Mumbai, closest to
India), and output format `json`. Check it:

```bash
aws sts get-caller-identity
```

```bash
export AWS_REGION=ap-south-1
export AZ=ap-south-1a
```

## A2. VPC: your private network

**Concepts**

| Thing | What it is |
|-------|------------|
| **VPC** | Your own isolated network in AWS. We give it the address range `10.0.0.0/16` (65k private IPs). |
| **Subnet** | A slice of the VPC in one availability zone. `10.0.1.0/24` = 256 addresses. |
| **Internet Gateway (IGW)** | The door between the VPC and the internet. |
| **Route table** | Rules for where traffic goes. `0.0.0.0/0 → IGW` means "everything not local goes to the internet". A subnet with that route is a **public subnet**. |

Create the VPC:

```bash
export VPC_ID=$(aws ec2 create-vpc --cidr-block 10.0.0.0/16 --tag-specifications 'ResourceType=vpc,Tags=[{Key=Name,Value=go-todo-vpc}]' --query Vpc.VpcId --output text)
echo $VPC_ID
```

```bash
aws ec2 modify-vpc-attribute --vpc-id $VPC_ID --enable-dns-hostnames '{"Value":true}'
```

Create the public subnet and make VMs in it get a public IP:

```bash
export SUBNET_ID=$(aws ec2 create-subnet --vpc-id $VPC_ID --cidr-block 10.0.1.0/24 --availability-zone $AZ --tag-specifications 'ResourceType=subnet,Tags=[{Key=Name,Value=go-todo-public}]' --query Subnet.SubnetId --output text)
echo $SUBNET_ID
```

```bash
aws ec2 modify-subnet-attribute --subnet-id $SUBNET_ID --map-public-ip-on-launch
```

Create the Internet Gateway and attach it to the VPC:

```bash
export IGW_ID=$(aws ec2 create-internet-gateway --tag-specifications 'ResourceType=internet-gateway,Tags=[{Key=Name,Value=go-todo-igw}]' --query InternetGateway.InternetGatewayId --output text)
aws ec2 attach-internet-gateway --internet-gateway-id $IGW_ID --vpc-id $VPC_ID
echo $IGW_ID
```

Create a route table that sends internet traffic to the IGW, and attach it to the subnet:

```bash
export RTB_ID=$(aws ec2 create-route-table --vpc-id $VPC_ID --tag-specifications 'ResourceType=route-table,Tags=[{Key=Name,Value=go-todo-public-rt}]' --query RouteTable.RouteTableId --output text)
aws ec2 create-route --route-table-id $RTB_ID --destination-cidr-block 0.0.0.0/0 --gateway-id $IGW_ID
aws ec2 associate-route-table --route-table-id $RTB_ID --subnet-id $SUBNET_ID
```

## A3. Firewall: Security Group

A **Security Group** is a stateful firewall attached to the VM. It denies all
inbound traffic by default; we open only what's needed:

| Port | From | Why |
|------|------|-----|
| 22 (SSH) | **your IP only** | so only you can log in |
| 80 (HTTP) | anywhere | website + Let's Encrypt validation |
| 443 (HTTPS) | anywhere | website |

Ports 8080–8083, 5432 and 4222 are **not** opened, so the services are invisible from the internet.

```bash
export SG_ID=$(aws ec2 create-security-group --group-name go-todo-web --description "Go-ToDo web server" --vpc-id $VPC_ID --query GroupId --output text)
echo $SG_ID
```

```bash
aws ec2 authorize-security-group-ingress --group-id $SG_ID --protocol tcp --port 22 --cidr $MY_IP/32
aws ec2 authorize-security-group-ingress --group-id $SG_ID --protocol tcp --port 80 --cidr 0.0.0.0/0
aws ec2 authorize-security-group-ingress --group-id $SG_ID --protocol tcp --port 443 --cidr 0.0.0.0/0
```

If your home IP changes later, SSH will stop working. Add the new IP with the
same port-22 command.

## A4. SSH key pair

```bash
aws ec2 create-key-pair --key-name go-todo-key --key-type ed25519 --query KeyMaterial --output text > ~/.ssh/go-todo-key.pem
chmod 400 ~/.ssh/go-todo-key.pem
```

This private key is the only way in. Don't commit it or lose it.

## A5. The VM (EC2 instance)

Look up the latest official Ubuntu 24.04 image (Canonical publishes its ID):

```bash
export AMI_ID=$(aws ssm get-parameters --names /aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id --query 'Parameters[0].Value' --output text)
echo $AMI_ID
```

Launch it: `t3.small` (2 vCPU, 2 GB RAM), 20 GB disk, and IMDSv2 required (a
security best practice):

```bash
export INSTANCE_ID=$(aws ec2 run-instances --image-id $AMI_ID --instance-type t3.small --key-name go-todo-key --subnet-id $SUBNET_ID --security-group-ids $SG_ID --block-device-mappings '[{"DeviceName":"/dev/sda1","Ebs":{"VolumeSize":20,"VolumeType":"gp3"}}]' --metadata-options HttpTokens=required --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=go-todo}]' --query 'Instances[0].InstanceId' --output text)
echo $INSTANCE_ID
```

```bash
aws ec2 wait instance-running --instance-ids $INSTANCE_ID
```

## A6. Static public IP (Elastic IP)

A normal public IP changes when the VM stops. An **Elastic IP** stays the same,
which you need for DNS:

```bash
export EIP_ALLOC=$(aws ec2 allocate-address --domain vpc --query AllocationId --output text)
aws ec2 associate-address --instance-id $INSTANCE_ID --allocation-id $EIP_ALLOC
export VM_IP=$(aws ec2 describe-addresses --allocation-ids $EIP_ALLOC --query 'Addresses[0].PublicIp' --output text)
echo $VM_IP
```

## A7. Log in

```bash
ssh -i ~/.ssh/go-todo-key.pem ubuntu@$VM_IP
```

Type `yes` the first time. You're now on the VM as user `ubuntu` (it has `sudo`).
**Continue with Part C.**

---

# Part B — GCP (instead of Part A)

The same steps with Google Cloud. GCP calls the pieces slightly differently:

| AWS | GCP |
|-----|-----|
| VPC + subnet | VPC network (custom mode) + subnet |
| Internet Gateway + route table | **automatic**: a custom VPC gets a default internet route |
| Security Group | **Firewall rules** on the network, applied to VMs by **network tag** |
| Key pair | `gcloud compute ssh` creates and uploads keys for you |
| Elastic IP | Static external IP address |
| `t3.small` | `e2-small` |

## B1. Project and CLI

1. Create a project at https://console.cloud.google.com (e.g. `go-todo-prod`) and **enable billing** for it.
2. Log in and select it (`gcloud` is already installed):

```bash
gcloud init
```

```bash
gcloud config set project go-todo-prod
gcloud config set compute/region asia-south1
gcloud config set compute/zone asia-south1-a
gcloud services enable compute.googleapis.com
```

## B2. VPC and subnet

```bash
gcloud compute networks create go-todo-vpc --subnet-mode=custom
gcloud compute networks subnets create go-todo-public --network=go-todo-vpc --region=asia-south1 --range=10.0.1.0/24
```

## B3. Firewall rules

GCP denies inbound traffic by default. These rules allow SSH from your IP only,
and web traffic from anywhere, for VMs tagged `go-todo`:

```bash
gcloud compute firewall-rules create go-todo-allow-ssh --network=go-todo-vpc --direction=INGRESS --allow=tcp:22 --source-ranges=$MY_IP/32 --target-tags=go-todo
gcloud compute firewall-rules create go-todo-allow-web --network=go-todo-vpc --direction=INGRESS --allow=tcp:80,tcp:443 --source-ranges=0.0.0.0/0 --target-tags=go-todo
```

## B4. Static IP and VM

```bash
gcloud compute addresses create go-todo-ip --region=asia-south1
export VM_IP=$(gcloud compute addresses describe go-todo-ip --region=asia-south1 --format='value(address)')
echo $VM_IP
```

```bash
gcloud compute instances create go-todo --zone=asia-south1-a --machine-type=e2-small --subnet=go-todo-public --address=go-todo-ip --tags=go-todo --image-family=ubuntu-2404-lts-amd64 --image-project=ubuntu-os-cloud --boot-disk-size=20GB --boot-disk-type=pd-balanced
```

## B5. Log in

```bash
gcloud compute ssh go-todo --zone=asia-south1-a
```

The first time, it generates an SSH key and uploads it. You're logged in as
your own username, with `sudo`. **Continue with Part C.**

---

# Part C — The VM (same for AWS and GCP)

All commands from here run **on the VM**. On the VM `#` comments would be fine
(it's bash), but the blocks stay comment-free anyway.

## C1. Update Ubuntu and add swap

A 2 GB VM can run out of memory while compiling Go and bundling React. Adding
2 GB of swap prevents that:

```bash
sudo apt update && sudo apt upgrade -y
```

```bash
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
free -h
```

Set the timezone (optional; logs are UTC JSON anyway):

```bash
sudo timedatectl set-timezone Asia/Kolkata
```

If `apt upgrade` installed a new kernel, reboot and SSH back in: `sudo reboot`.

## C2. Install the software

| Package | Why |
|---------|-----|
| `git`, `make`, `curl` | get and build the code |
| `nginx` | public web server + reverse proxy |
| `supervisor` | runs and restarts the Go services |
| `ufw` | firewall on the VM itself (second layer after the cloud firewall) |
| Docker | runs Postgres + NATS |
| Go 1.27.3 | compiles the services |
| Node 22 | builds the React app |

```bash
sudo apt install -y git make curl ca-certificates nginx supervisor ufw
```

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl enable --now docker
docker --version
```

```bash
curl -fsSL https://go.dev/dl/go1.27.3.linux-amd64.tar.gz -o /tmp/go.tgz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
. /etc/profile.d/go.sh
go version
```

```bash
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs
node -v
```

```bash
sudo systemctl enable --now supervisor nginx
systemctl status supervisor nginx --no-pager
```

## C3. VM firewall (ufw)

The cloud firewall already blocks everything except 22/80/443. `ufw` repeats
that on the VM, so a mistake in one layer doesn't expose anything.
**Allow SSH first**, or you'll lock yourself out:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 'Nginx Full'
sudo ufw --force enable
sudo ufw status verbose
```

> Docker-published ports **bypass ufw**. That's why our Postgres and NATS are
> published only on `127.0.0.1` (see `docker-compose.infra.yml`), not `0.0.0.0`.

## C4. App user, folders and code

The services run as a dedicated user `gotodo` with no login and no sudo. If a
service is ever compromised, the attacker can't touch the rest of the system.

```bash
sudo adduser --system --group --home /home/gotodo gotodo
sudo mkdir -p /opt/go-todo /var/log/go-todo /etc/go-todo
sudo chown gotodo:gotodo /opt/go-todo /var/log/go-todo
```

```bash
sudo -u gotodo git clone https://github.com/PRASHMANE/Go-ToDo.git /opt/go-todo
ls /opt/go-todo
```

If the repo is **private**, create a GitHub *fine-grained personal access token*
with read-only "Contents" access to this repo, and clone with
`https://<TOKEN>@github.com/PRASHMANE/Go-ToDo.git`.

| Path | Contents |
|------|----------|
| `/opt/go-todo` | the code, `bin/` (compiled services), `frontend/dist/` (built React) |
| `/etc/go-todo` | secrets (`*.env`), readable only by root and `gotodo` |
| `/var/log/go-todo` | one log file per service (written by supervisord) |

## C5. Secrets (`/etc/go-todo/*.env`)

Each service reads its config from environment variables (12-factor, Phase B1).
On the VM, those come from one file per service:

```bash
cd /opt/go-todo/deployments/prod/env
for f in infra auth todo activity gateway; do sudo cp $f.env.example /etc/go-todo/$f.env; done
ls /etc/go-todo
```

Generate random values. Run each command as many times as you need values:

```bash
openssl rand -hex 24
```

```bash
openssl rand -hex 48
```

Now fill in the files (`nano`: edit, then Ctrl+O to save, Enter, Ctrl+X to exit):

```bash
sudo nano /etc/go-todo/infra.env
```

| File | What to set |
|------|-------------|
| `infra.env` | `POSTGRES_PASSWORD`, `AUTH_DB_PASSWORD`, `TODO_DB_PASSWORD`, `ACTIVITY_DB_PASSWORD`: 4 different `openssl rand -hex 24` values |
| `auth.env` | in `DATABASE_URL` replace `AUTH_DB_PASSWORD` with the value from `infra.env`; `JWT_SECRET` = an `openssl rand -hex 48` value |
| `todo.env` | in `DATABASE_URL` replace `TODO_DB_PASSWORD` |
| `activity.env` | in `DATABASE_URL` replace `ACTIVITY_DB_PASSWORD` |
| `gateway.env` | `JWT_SECRET` = **the same value as in `auth.env`**; `CORS_ORIGINS=http://YOUR_VM_IP` for now (later `https://your-domain`) |

```bash
sudo nano /etc/go-todo/auth.env
sudo nano /etc/go-todo/todo.env
sudo nano /etc/go-todo/activity.env
sudo nano /etc/go-todo/gateway.env
```

Lock them down:

```bash
sudo chown root:gotodo /etc/go-todo/*.env
sudo chmod 640 /etc/go-todo/*.env
ls -l /etc/go-todo
```

## C6. Postgres + NATS (Docker)

```bash
cd /opt/go-todo
sudo docker compose --env-file /etc/go-todo/infra.env -f deployments/prod/docker-compose.infra.yml up -d --wait
sudo docker compose -f deployments/prod/docker-compose.infra.yml ps
```

On this first start, `postgres-init.sh` creates `auth_db`, `todo_db` and
`activity_db`, each with its own user and the password from `infra.env`.

Check that the ports are private (bound to `127.0.0.1`, not `0.0.0.0`):

```bash
sudo ss -ltnp | grep -E '5432|4222'
```

## C7. The supervisor config: where the start commands live

**This is the file that starts all the servers:**
**`/etc/supervisor/conf.d/go-todo.conf`**
(the source is in the repo: `deployments/prod/supervisor/go-todo.conf`).

How it fits together:

1. `systemd` starts **supervisord** at boot.
2. supervisord reads every `*.conf` in `/etc/supervisor/conf.d/`.
3. For each `[program:…]` block it runs the `command=` line, as the `user=`, and
   keeps it alive (`autorestart=true`).
4. **`supervisorctl`** is the remote control you type commands into. It talks to
   supervisord; it doesn't run anything itself.

One block, line by line:

```ini
[group:go-todo]
programs=auth-service,todo-service,activity-service,api-gateway

[program:todo-service]
command=/opt/go-todo/deployments/prod/run-with-env.sh /etc/go-todo/todo.env /opt/go-todo/bin/todo-service
directory=/opt/go-todo
user=gotodo
priority=100
autostart=true
autorestart=true
startsecs=3
startretries=10
stopsignal=TERM
stopwaitsecs=20
stopasgroup=true
killasgroup=true
redirect_stderr=true
stdout_logfile=/var/log/go-todo/todo-service.log
stdout_logfile_maxbytes=20MB
stdout_logfile_backups=5
```

| Line | Meaning |
|------|---------|
| `[group:go-todo]` | Groups the 4 programs, so `supervisorctl restart 'go-todo:*'` restarts all of them. |
| `command=` | **The start command.** `run-with-env.sh` loads `/etc/go-todo/todo.env` into the environment, then `exec`s the binary. |
| `directory=` | Working directory. |
| `user=gotodo` | Runs as the unprivileged app user, never root. |
| `priority=100` | Start order: services (100) before the gateway (200). |
| `autostart=true` | Start when supervisord starts (i.e. at boot). |
| `autorestart=true` | Restart if it crashes. |
| `startsecs=3` | Must stay up 3 s to count as "started". |
| `startretries=10` | Keep retrying, e.g. while Postgres is still starting after a reboot. |
| `stopsignal=TERM` | Stop with SIGTERM → our graceful shutdown (Phase B1) finishes in-flight requests. |
| `stopwaitsecs=20` | If it hasn't exited after 20 s, kill it. |
| `stopasgroup` / `killasgroup` | Signal the whole process group, so no orphans are left. |
| `redirect_stderr=true` | Errors go to the same log file. |
| `stdout_logfile*` | Log file, rotated at 20 MB, with 5 old files kept. |

Why the wrapper? supervisord could set variables with `environment=KEY=val`,
but then the secrets would sit in the supervisor config. The wrapper keeps them
in `/etc/go-todo` with tight permissions. Because it uses `exec`, the Go process
*replaces* the wrapper, so supervisord's stop signal reaches Go directly.

Install it:

```bash
sudo cp /opt/go-todo/deployments/prod/supervisor/go-todo.conf /etc/supervisor/conf.d/go-todo.conf
```

## C8. Build, migrate and start (`deploy.sh`)

`deployments/prod/deploy.sh` does a full deploy:

1. `git pull` (as `gotodo`)
2. `make release` → `bin/api-gateway`, `bin/auth-service`, `bin/todo-service`, `bin/activity-service`
3. `npm ci && npm run build` → `frontend/dist`
4. starts Postgres/NATS if needed, runs DB migrations (with the `migrate/migrate` Docker image)
5. `supervisorctl reread` + `update` + `restart 'go-todo:*'`, then prints status and health

```bash
sudo /opt/go-todo/deployments/prod/deploy.sh
```

The first run takes a few minutes (it downloads Go modules and npm packages). At
the end you should see:

```
go-todo:activity-service   RUNNING   pid 2345, uptime 0:00:04
go-todo:api-gateway        RUNNING   pid 2346, uptime 0:00:04
go-todo:auth-service       RUNNING   pid 2347, uptime 0:00:04
go-todo:todo-service       RUNNING   pid 2348, uptime 0:00:04
{"status":"ok"}
{"checks":{"db":"ok"},"status":"ready"}
{"checks":{"db":"ok","nats":"ok"},"status":"ready"}
{"checks":{"db":"ok","nats":"ok"},"status":"ready"}
```

Test the gateway locally on the VM (not reachable from outside, which is correct):

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s -X POST http://127.0.0.1:8080/api/v1/auth/register -H 'Content-Type: application/json' -d '{"email":"me@example.com","name":"Me","password":"password123"}'
```

## C9. nginx: the public front door

**The file:** `/etc/nginx/sites-available/go-todo`
(the source is in the repo: `deployments/prod/nginx/go-todo.conf`).

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name todo.example.com;

    root /opt/go-todo/frontend/dist;
    index index.html;
    client_max_body_size 1m;

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 30s;
    }

    location /assets/ {
        expires 1y;
        try_files $uri =404;
    }

    location / {
        expires -1;
        try_files $uri $uri/ /index.html;
    }

    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript image/svg+xml;
}
```

| Part | Meaning |
|------|---------|
| `listen 80` / `[::]:80` | Accept HTTP on IPv4 and IPv6. certbot adds the 443 (HTTPS) block later. |
| `server_name` | Your domain. For a first test with no domain, use `_` (matches anything). |
| `root /opt/go-todo/frontend/dist` | Where the built React files are. |
| `client_max_body_size 1m` | Reject huge request bodies early. |
| `location /api/` + `proxy_pass http://127.0.0.1:8080` | **Every `/api/...` request goes to the API gateway.** The gateway then routes it: `/api/v1/auth/*` → auth-service, `/api/v1/todos/*` → todo-service, `/api/v1/activity` → activity-service. No trailing `/` after `8080`, so the path is passed unchanged. |
| `proxy_set_header X-Forwarded-For …` | Tells the gateway the real client IP. The gateway trusts it because `TRUSTED_PROXIES=127.0.0.1`, and uses it for rate limiting. |
| `location /assets/` + `expires 1y` | Vite's hashed JS/CSS files can be cached for a year. |
| `location /` + `try_files … /index.html` | **SPA fallback**: `/login` isn't a real file, so nginx returns `index.html` and React Router shows the right page. |
| `expires -1` | Never cache `index.html`, so users always get the newest app after a deploy. |
| `add_header …` | Security headers. They're inherited by the locations because those use `expires`, not their own `add_header`. |
| `gzip` | Compress text responses. |

Set your domain (or `_` for IP-only testing):

```bash
sudo nano /opt/go-todo/deployments/prod/nginx/go-todo.conf
```

Enable the site, remove the default one, test and reload:

```bash
sudo cp /opt/go-todo/deployments/prod/nginx/go-todo.conf /etc/nginx/sites-available/go-todo
sudo ln -sf /etc/nginx/sites-available/go-todo /etc/nginx/sites-enabled/go-todo
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl reload nginx
```

`nginx` needs to read `frontend/dist`. Check it can:

```bash
sudo -u www-data ls /opt/go-todo/frontend/dist
```

Now open **`http://YOUR_VM_IP`** in your browser. Register, log in and add todos. 🎉

## C10. Domain + HTTPS

1. At your domain registrar (or Route 53 / Cloud DNS), add an **A record**:
   `todo.example.com → YOUR_VM_IP`. Wait until this shows the IP: `dig +short todo.example.com`
2. Make sure `server_name todo.example.com;` is set in `/etc/nginx/sites-available/go-todo`.
3. Get a free certificate. certbot edits the nginx file and adds the 443 block and an HTTP→HTTPS redirect:

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d todo.example.com
```

```bash
sudo certbot renew --dry-run
```

4. Update CORS for the HTTPS origin and restart the gateway:

```bash
sudo nano /etc/go-todo/gateway.env
sudo supervisorctl restart go-todo:api-gateway
```

Open **`https://todo.example.com`**. 🔒

---

## C11. Everyday supervisorctl commands

| Task | Command |
|------|---------|
| Status of everything | `sudo supervisorctl status` |
| Restart all 4 services | `sudo supervisorctl restart 'go-todo:*'` |
| Restart one | `sudo supervisorctl restart go-todo:todo-service` |
| Stop / start one | `sudo supervisorctl stop go-todo:todo-service` / `start …` |
| Follow a service's log | `sudo supervisorctl tail -f go-todo:api-gateway` |
| Read log files | `sudo tail -f /var/log/go-todo/todo-service.log` |
| You edited `go-todo.conf` | `sudo supervisorctl reread && sudo supervisorctl update` |
| You edited an `.env` file | `sudo supervisorctl restart go-todo:<service>` |
| Supervisor itself | `sudo systemctl status supervisor` |
| nginx config changed | `sudo nginx -t && sudo systemctl reload nginx` |
| nginx logs | `sudo tail -f /var/log/nginx/access.log /var/log/nginx/error.log` |
| Database shell | `sudo docker exec -it go-todo-infra-postgres-1 psql -U postgres` |

## C12. Deploy a new version

On your Mac:

```bash
git push origin main
```

On the VM:

```bash
sudo /opt/go-todo/deployments/prod/deploy.sh
```

## C13. Check that everything survives a reboot

```bash
sudo reboot
```

SSH back in after ~1 minute:

```bash
sudo supervisorctl status
sudo docker ps
curl -s http://127.0.0.1:8080/healthz
```

systemd starts Docker and supervisord; Docker restarts Postgres/NATS
(`restart: unless-stopped`); supervisord starts the Go services. If one starts
before Postgres is ready, it exits and supervisord retries it (`startretries=10`).

## C14. Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| SSH times out | Your IP changed. Allow the new `$MY_IP` on port 22 (A3 / B3). |
| Browser can't connect to `http://VM_IP` | Cloud firewall (A3/B3) or ufw (C3) missing port 80; `sudo systemctl status nginx`. |
| nginx **502 Bad Gateway** | The gateway isn't running: `sudo supervisorctl status go-todo:api-gateway`. |
| JSON `{"code":"bad_gateway"}` | The gateway is up, but a service behind it isn't: check `supervisorctl status`. |
| Service in `FATAL` / `BACKOFF` | `sudo supervisorctl tail go-todo:<name>`: usually a wrong `.env` value (`JWT_SECRET is not set`, `password authentication failed`). |
| `spawn error` | `ls /opt/go-todo/bin` is empty (re-run `deploy.sh`), or the wrapper lost its execute bit: `sudo chmod +x /opt/go-todo/deployments/prod/*.sh`. |
| `permission denied` on `.env` | `sudo chown root:gotodo /etc/go-todo/*.env && sudo chmod 640 /etc/go-todo/*.env` |
| Login works, but then every request is 401 | `JWT_SECRET` differs between `auth.env` and `gateway.env`. |
| Build killed / `signal: killed` | Out of memory: check the swap from C1 (`free -h`). |
| certbot fails | The DNS A record doesn't point to the VM yet, or port 80 is closed. |

---

# Part D — Delete everything (stop paying)

## AWS

```bash
aws ec2 terminate-instances --instance-ids $INSTANCE_ID
aws ec2 wait instance-terminated --instance-ids $INSTANCE_ID
aws ec2 release-address --allocation-id $EIP_ALLOC
aws ec2 delete-security-group --group-id $SG_ID
aws ec2 delete-key-pair --key-name go-todo-key
aws ec2 detach-internet-gateway --internet-gateway-id $IGW_ID --vpc-id $VPC_ID
aws ec2 delete-internet-gateway --internet-gateway-id $IGW_ID
aws ec2 delete-subnet --subnet-id $SUBNET_ID
aws ec2 delete-route-table --route-table-id $RTB_ID
aws ec2 delete-vpc --vpc-id $VPC_ID
```

If you closed the terminal, find the IDs by name:

```bash
aws ec2 describe-vpcs --filters Name=tag:Name,Values=go-todo-vpc --query 'Vpcs[].VpcId' --output text
aws ec2 describe-instances --filters Name=tag:Name,Values=go-todo --query 'Reservations[].Instances[].InstanceId' --output text
aws ec2 describe-addresses --query 'Addresses[].[AllocationId,PublicIp]' --output text
```

## GCP

```bash
gcloud compute instances delete go-todo --zone=asia-south1-a --quiet
gcloud compute addresses delete go-todo-ip --region=asia-south1 --quiet
gcloud compute firewall-rules delete go-todo-allow-ssh go-todo-allow-web --quiet
gcloud compute networks subnets delete go-todo-public --region=asia-south1 --quiet
gcloud compute networks delete go-todo-vpc --quiet
```

---

## Where to go next

- Put Postgres on a managed service (**RDS** / **Cloud SQL**) with automatic backups.
- Put the VM in a **private subnet** behind a load balancer (**ALB** / **Cloud Load Balancing**), with a NAT gateway for outbound traffic.
- Build the binaries in CI (GitHub Actions) and copy them to the VM, instead of compiling on the server.
- Replace supervisord with **systemd units**, or run everything on **Kubernetes** (EKS / GKE).

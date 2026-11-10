# Go-ToDo — Complete Deployment Guide (from scratch)

This one file has **everything** needed to put Go-ToDo on the internet: the
cloud network, the firewall, the Ubuntu VM, every command, and the **full
content of every config file**. Follow it top to bottom.

**What you end up with**

```
                        ┌──────────────────────── VPC 10.0.0.0/16 ────────────────────────┐
                        │  public subnet 10.0.1.0/24                                       │
Internet ──▶ Firewall ──┼─▶ VM  Ubuntu 24.04, static public IP                             │
 80/443 open to all,    │     ufw: 22, 80, 443                                             │
 22 only from your IP   │     nginx :80/:443 ─┬─ /        → /opt/go-todo/frontend/dist      │
                        │                     └─ /api/*   → 127.0.0.1:8080  api-gateway     │
                        │     supervisord ─▶ api-gateway :8080 ─┬─▶ auth-service     :8081  │
                        │                                       ├─▶ todo-service     :8082  │
                        │                                       └─▶ activity-service :8083  │
                        │     docker ─▶ postgres 127.0.0.1:5432 · nats 127.0.0.1:4222      │
                        └──────────────────────────────────────────────────────────────────┘
```

- The **browser only talks to nginx**. nginx serves the React app and sends every
  `/api/...` request to the **API gateway (port 8080)**.
- The gateway checks the JWT and forwards by path: `/api/v1/auth/*` → auth,
  `/api/v1/todos/*` → todo, `/api/v1/activity` → activity.
- **supervisord** starts the 4 Go programs at boot and restarts them if they crash.
- Only ports 22, 80 and 443 are reachable from the internet.

## Contents

| Part | What |
|------|------|
| [0](#part-0--how-it-all-fits-together) | How it all fits together: every file and where it lives |
| [1](#part-1--prepare-on-your-mac) | Prepare on your Mac |
| [2A](#part-2a--create-the-network-and-vm-on-aws) | AWS: VPC, subnet, internet gateway, route table, security group, key, VM, Elastic IP |
| [2B](#part-2b--create-the-network-and-vm-on-gcp-instead-of-2a) | GCP: the same with `gcloud` (instead of 2A) |
| [3](#part-3--prepare-the-ubuntu-vm) | Ubuntu: updates, swap, packages, Docker, Go, Node, ufw firewall |
| [4](#part-4--app-user-folders-and-code) | App user, folders, clone the code |
| [5](#part-5--secrets-etcgo-todoenv) | Secrets: generate passwords, write the 5 env files |
| [6](#part-6--postgres--nats-docker) | Postgres + NATS in Docker |
| [7](#part-7--the-start-wrapper-usrlocalbingo-todo-run) | The start wrapper |
| [8](#part-8--supervisor-the-file-that-starts-all-the-servers) | **Supervisor: the file that starts all the servers** |
| [9](#part-9--the-deploy-script-build--migrate--start) | Deploy script: build, migrate, start |
| [10](#part-10--nginx-the-public-front-door) | **nginx config** |
| [11](#part-11--domain--https) | Domain + HTTPS |
| [12](#part-12--everyday-commands) | Everyday commands (`supervisorctl` and friends) |
| [13](#part-13--reboot-test) | Reboot test |
| [14](#part-14--troubleshooting) | Troubleshooting |
| [15](#part-15--delete-everything) | Delete everything (stop paying) |

> **Copy rule.** Copy one grey block at a time. Blocks marked **(Mac)** run in
> the terminal on your Mac. Blocks marked **(VM)** run after you SSH in. The
> Mac blocks contain no `# comments` (zsh would treat them as commands).

> **Hostinger / DigitalOcean / any VPS?** Skip Part 2 (your provider already
> gave you a VM with a public IP). Set the provider's firewall to allow only
> 22/80/443, SSH in, and start at Part 3. If you log in as `root`, the `sudo`
> in front of commands is harmless.

> **Cost:** a 2 GB VM (`t3.small` / `e2-small`) is roughly US$15–20 per month.
> Part 15 deletes everything.

---

## Part 0 — How it all fits together

Every file the server uses, where it lives, and who reads it:

| File on the VM | Created in | Read by | Purpose |
|----------------|------------|---------|---------|
| `/opt/go-todo/` | Part 4 | everyone | The code (`git clone`). After the build: `bin/` (4 binaries) and `frontend/dist/` (the built React app). |
| `/etc/go-todo/infra.env` | Part 5 | Docker | Postgres passwords |
| `/etc/go-todo/auth.env` | Part 5 | auth-service | Port, DB URL, JWT secret |
| `/etc/go-todo/todo.env` | Part 5 | todo-service | Port, DB URL, NATS URL |
| `/etc/go-todo/activity.env` | Part 5 | activity-service | Port, DB URL, NATS URL |
| `/etc/go-todo/gateway.env` | Part 5 | api-gateway | Port, service URLs, JWT secret, CORS, rate limit |
| `/etc/go-todo/docker-compose.infra.yml` | Part 6 | Docker | Runs Postgres + NATS on 127.0.0.1 |
| `/etc/go-todo/postgres-init.sh` | Part 6 | Postgres (first start) | Creates the 3 databases + users |
| `/usr/local/bin/go-todo-run` | Part 7 | supervisord | Loads an env file, then starts a binary |
| **`/etc/supervisor/conf.d/go-todo.conf`** | **Part 8** | **supervisord** | **The start commands for all 4 servers** |
| `/usr/local/bin/go-todo-deploy` | Part 9 | you | pull → build → migrate → restart |
| **`/etc/nginx/sites-available/go-todo`** | **Part 10** | **nginx** | **Website + `/api` → gateway** |
| `/var/log/go-todo/*.log` | Part 4 | you | One log per service, written by supervisord |

**Boot sequence:** systemd starts Docker and **supervisord** → Docker starts
Postgres + NATS → supervisord reads `/etc/supervisor/conf.d/go-todo.conf` and
runs the `command=` of each program → each command runs `go-todo-run`, which
loads `/etc/go-todo/<service>.env` and starts `/opt/go-todo/bin/<service>` →
nginx accepts traffic on 80/443.

(The same files are also kept in the repo under `deployments/prod/`, for reference.)

---

## Part 1 — Prepare on your Mac

Push the code, so the VM can clone it. **(Mac)**

```bash
cd ~/Documents/Go_ToDo_Project
git push origin main --tags
```

Find your public IP. The firewall will allow SSH only from it. **(Mac)**

```bash
export MY_IP=$(curl -s https://checkip.amazonaws.com)
echo $MY_IP
```

Keep using **this same terminal window** for Part 2: the commands save IDs in
variables (`$VPC_ID`, `$SG_ID`, …) that later commands need.

---

## Part 2A — Create the network and VM on AWS

### 2A.1 Account and CLI

1. Create an account at https://aws.amazon.com and enable **MFA** on the root user.
2. **IAM → Users → Create user** (e.g. `prashanth-admin`), attach the
   `AdministratorAccess` policy, then **Security credentials → Create access key → CLI**.
3. Configure the CLI. Enter the key, the secret, region `ap-south-1` (Mumbai) and output `json`. **(Mac)**

```bash
aws configure
```

```bash
aws sts get-caller-identity
export AZ=ap-south-1a
```

### 2A.2 VPC: your private network

| Piece | Meaning |
|-------|---------|
| **VPC** `10.0.0.0/16` | Your own isolated network (65,536 private addresses). |
| **Subnet** `10.0.1.0/24` | A slice of the VPC in one availability zone. |
| **Internet Gateway** | The door between the VPC and the internet. |
| **Route table** `0.0.0.0/0 → IGW` | "Send all non-local traffic to the internet". A subnet with this route is a **public subnet**. |

Create the VPC. **(Mac)**

```bash
export VPC_ID=$(aws ec2 create-vpc --cidr-block 10.0.0.0/16 --tag-specifications 'ResourceType=vpc,Tags=[{Key=Name,Value=go-todo-vpc}]' --query Vpc.VpcId --output text)
aws ec2 modify-vpc-attribute --vpc-id $VPC_ID --enable-dns-hostnames '{"Value":true}'
echo $VPC_ID
```

Create the public subnet; VMs in it get a public IP automatically. **(Mac)**

```bash
export SUBNET_ID=$(aws ec2 create-subnet --vpc-id $VPC_ID --cidr-block 10.0.1.0/24 --availability-zone $AZ --tag-specifications 'ResourceType=subnet,Tags=[{Key=Name,Value=go-todo-public}]' --query Subnet.SubnetId --output text)
aws ec2 modify-subnet-attribute --subnet-id $SUBNET_ID --map-public-ip-on-launch
echo $SUBNET_ID
```

Create the Internet Gateway and attach it. **(Mac)**

```bash
export IGW_ID=$(aws ec2 create-internet-gateway --tag-specifications 'ResourceType=internet-gateway,Tags=[{Key=Name,Value=go-todo-igw}]' --query InternetGateway.InternetGatewayId --output text)
aws ec2 attach-internet-gateway --internet-gateway-id $IGW_ID --vpc-id $VPC_ID
echo $IGW_ID
```

Create the route table, add the internet route, and attach it to the subnet. **(Mac)**

```bash
export RTB_ID=$(aws ec2 create-route-table --vpc-id $VPC_ID --tag-specifications 'ResourceType=route-table,Tags=[{Key=Name,Value=go-todo-public-rt}]' --query RouteTable.RouteTableId --output text)
aws ec2 create-route --route-table-id $RTB_ID --destination-cidr-block 0.0.0.0/0 --gateway-id $IGW_ID
aws ec2 associate-route-table --route-table-id $RTB_ID --subnet-id $SUBNET_ID
echo $RTB_ID
```

### 2A.3 Firewall: Security Group

A Security Group blocks **all** inbound traffic until you allow it:

| Port | Allowed from | Why |
|------|--------------|-----|
| 22 | your IP only | SSH, so only you can log in |
| 80 | anywhere | HTTP + Let's Encrypt validation |
| 443 | anywhere | HTTPS |

8080–8083 (Go services), 5432 (Postgres) and 4222 (NATS) are **not** opened.

**(Mac)**

```bash
export SG_ID=$(aws ec2 create-security-group --group-name go-todo-web --description "Go-ToDo web server" --vpc-id $VPC_ID --query GroupId --output text)
aws ec2 authorize-security-group-ingress --group-id $SG_ID --protocol tcp --port 22 --cidr $MY_IP/32
aws ec2 authorize-security-group-ingress --group-id $SG_ID --protocol tcp --port 80 --cidr 0.0.0.0/0
aws ec2 authorize-security-group-ingress --group-id $SG_ID --protocol tcp --port 443 --cidr 0.0.0.0/0
echo $SG_ID
```

### 2A.4 SSH key

**(Mac)**

```bash
aws ec2 create-key-pair --key-name go-todo-key --key-type ed25519 --query KeyMaterial --output text > ~/.ssh/go-todo-key.pem
chmod 400 ~/.ssh/go-todo-key.pem
```

### 2A.5 The VM

Look up the latest official Ubuntu 24.04 image, then launch a `t3.small`
(2 vCPU, 2 GB) with a 20 GB disk. **(Mac)**

```bash
export AMI_ID=$(aws ssm get-parameters --names /aws/service/canonical/ubuntu/server/24.04/stable/current/amd64/hvm/ebs-gp3/ami-id --query 'Parameters[0].Value' --output text)
echo $AMI_ID
```

```bash
export INSTANCE_ID=$(aws ec2 run-instances --image-id $AMI_ID --instance-type t3.small --key-name go-todo-key --subnet-id $SUBNET_ID --security-group-ids $SG_ID --block-device-mappings '[{"DeviceName":"/dev/sda1","Ebs":{"VolumeSize":20,"VolumeType":"gp3"}}]' --metadata-options HttpTokens=required --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=go-todo}]' --query 'Instances[0].InstanceId' --output text)
aws ec2 wait instance-running --instance-ids $INSTANCE_ID
echo $INSTANCE_ID
```

### 2A.6 Static IP (Elastic IP)

A normal public IP changes on stop/start; an Elastic IP never does. **(Mac)**

```bash
export EIP_ALLOC=$(aws ec2 allocate-address --domain vpc --query AllocationId --output text)
aws ec2 associate-address --instance-id $INSTANCE_ID --allocation-id $EIP_ALLOC
export VM_IP=$(aws ec2 describe-addresses --allocation-ids $EIP_ALLOC --query 'Addresses[0].PublicIp' --output text)
echo $VM_IP
```

**Write down `$VM_IP`.** You need it in Part 5.

### 2A.7 Log in

**(Mac)**

```bash
ssh -i ~/.ssh/go-todo-key.pem ubuntu@$VM_IP
```

Answer `yes` the first time. You're now on the VM as `ubuntu` (with sudo). → **Part 3**

---

## Part 2B — Create the network and VM on GCP (instead of 2A)

| AWS | GCP equivalent |
|-----|----------------|
| VPC + subnet | VPC network (custom mode) + subnet |
| Internet Gateway + route table | Automatic: a GCP VPC gets a default internet route |
| Security Group | Firewall rules, applied to VMs by **network tag** |
| Key pair | `gcloud compute ssh` creates and uploads keys for you |
| Elastic IP | Static external IP |
| `t3.small` | `e2-small` |

1. https://console.cloud.google.com → create project `go-todo-prod` → enable **billing**.
2. Log in and select it. **(Mac)**

```bash
gcloud init
```

```bash
gcloud config set project go-todo-prod
gcloud config set compute/region asia-south1
gcloud config set compute/zone asia-south1-a
gcloud services enable compute.googleapis.com
```

VPC + subnet. **(Mac)**

```bash
gcloud compute networks create go-todo-vpc --subnet-mode=custom
gcloud compute networks subnets create go-todo-public --network=go-todo-vpc --region=asia-south1 --range=10.0.1.0/24
```

Firewall: SSH from your IP only, web from anywhere. **(Mac)**

```bash
gcloud compute firewall-rules create go-todo-allow-ssh --network=go-todo-vpc --direction=INGRESS --allow=tcp:22 --source-ranges=$MY_IP/32 --target-tags=go-todo
gcloud compute firewall-rules create go-todo-allow-web --network=go-todo-vpc --direction=INGRESS --allow=tcp:80,tcp:443 --source-ranges=0.0.0.0/0 --target-tags=go-todo
```

Static IP + VM. **(Mac)**

```bash
gcloud compute addresses create go-todo-ip --region=asia-south1
export VM_IP=$(gcloud compute addresses describe go-todo-ip --region=asia-south1 --format='value(address)')
echo $VM_IP
```

```bash
gcloud compute instances create go-todo --zone=asia-south1-a --machine-type=e2-small --subnet=go-todo-public --address=go-todo-ip --tags=go-todo --image-family=ubuntu-2404-lts-amd64 --image-project=ubuntu-os-cloud --boot-disk-size=20GB --boot-disk-type=pd-balanced
```

Log in. **(Mac)**

```bash
gcloud compute ssh go-todo --zone=asia-south1-a
```

→ **Part 3**

---

## Part 3 — Prepare the Ubuntu VM

Everything from here runs **on the VM**.

### 3.1 Update and reboot

**(VM)**

```bash
sudo apt update && sudo apt upgrade -y
sudo reboot
```

Wait ~30 seconds and SSH in again (same command as before).

### 3.2 Swap (prevents "out of memory" during builds on a 2 GB VM)

**(VM)**

```bash
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
free -h
```

### 3.3 Packages

| Package | Why |
|---------|-----|
| `git` `make` `curl` | get and build the code |
| `nginx` | public web server + reverse proxy |
| `supervisor` | starts and restarts the Go services |
| `ufw` | firewall on the VM itself |
| `jq` | read JSON in health checks |

**(VM)**

```bash
sudo apt install -y git make curl ca-certificates jq nginx supervisor ufw
sudo systemctl enable --now nginx supervisor
```

Docker (runs only Postgres + NATS). **(VM)**

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl enable --now docker
sudo docker run --rm hello-world
```

Go 1.27.3 (apt's version is too old). **(VM)**

```bash
curl -fsSL https://go.dev/dl/go1.27.3.linux-amd64.tar.gz -o /tmp/go.tgz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
. /etc/profile.d/go.sh
go version
```

Node 22 (builds the React app). **(VM)**

```bash
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt install -y nodejs
node -v
npm -v
```

### 3.4 Firewall on the VM (ufw)

This is a second layer behind the cloud firewall. **Allow SSH first**, or you'll
lock yourself out. **(VM)**

```bash
sudo ufw allow OpenSSH
sudo ufw allow 'Nginx Full'
sudo ufw --force enable
sudo ufw status verbose
```

> Docker-published ports **bypass ufw**. That's why Postgres and NATS are
> published on `127.0.0.1` only (Part 6). Never use `0.0.0.0` for them.

---

## Part 4 — App user, folders and code

The services run as `gotodo`, a system user with no password, no shell and no
sudo. **(VM)**

```bash
sudo adduser --system --group --home /home/gotodo gotodo
sudo mkdir -p /opt/go-todo /etc/go-todo /var/log/go-todo
sudo chown gotodo:gotodo /opt/go-todo /var/log/go-todo
```

Clone the code. **(VM)**

```bash
sudo -u gotodo git clone https://github.com/PRASHMANE/Go-ToDo.git /opt/go-todo
ls /opt/go-todo
```

Private repo? On GitHub create a **fine-grained personal access token** (this repo
only, permission *Contents: Read-only*) and clone with
`https://<TOKEN>@github.com/PRASHMANE/Go-ToDo.git`.

---

## Part 5 — Secrets (`/etc/go-todo/*.env`)

Every service reads its settings from environment variables. On the server they
come from one file per service. The block below **generates random passwords**
and writes all 5 files for you.

First set your address: your domain if you have one (e.g. `todo.example.com`),
otherwise the VM's public IP. **(VM)**

```bash
SITE=YOUR_DOMAIN_OR_VM_IP
```

Generate the secrets (they live in this shell until it closes, and in the files below). **(VM)**

```bash
PG_PW=$(openssl rand -hex 24)
AUTH_DB_PW=$(openssl rand -hex 24)
TODO_DB_PW=$(openssl rand -hex 24)
ACTIVITY_DB_PW=$(openssl rand -hex 24)
JWT_SECRET=$(openssl rand -hex 48)
echo "site=$SITE  jwt=${JWT_SECRET:0:8}…"
```

**`/etc/go-todo/infra.env`**: Postgres passwords (used by Docker). **(VM)**

```bash
sudo tee /etc/go-todo/infra.env > /dev/null <<EOF
POSTGRES_PASSWORD=$PG_PW
AUTH_DB_PASSWORD=$AUTH_DB_PW
TODO_DB_PASSWORD=$TODO_DB_PW
ACTIVITY_DB_PASSWORD=$ACTIVITY_DB_PW
EOF
```

**`/etc/go-todo/auth.env`** **(VM)**

```bash
sudo tee /etc/go-todo/auth.env > /dev/null <<EOF
PORT=8081
GIN_MODE=release
LOG_LEVEL=info
DATABASE_URL=postgres://auth_user:$AUTH_DB_PW@127.0.0.1:5432/auth_db?sslmode=disable
JWT_SECRET=$JWT_SECRET
JWT_ISSUER=go-todo-auth
JWT_TTL=24h
EOF
```

**`/etc/go-todo/todo.env`** **(VM)**

```bash
sudo tee /etc/go-todo/todo.env > /dev/null <<EOF
PORT=8082
GIN_MODE=release
LOG_LEVEL=info
DATABASE_URL=postgres://todo_user:$TODO_DB_PW@127.0.0.1:5432/todo_db?sslmode=disable
NATS_URL=nats://127.0.0.1:4222
EOF
```

**`/etc/go-todo/activity.env`** **(VM)**

```bash
sudo tee /etc/go-todo/activity.env > /dev/null <<EOF
PORT=8083
GIN_MODE=release
LOG_LEVEL=info
DATABASE_URL=postgres://activity_user:$ACTIVITY_DB_PW@127.0.0.1:5432/activity_db?sslmode=disable
NATS_URL=nats://127.0.0.1:4222
EOF
```

**`/etc/go-todo/gateway.env`**: the JWT secret **must equal** auth.env's. **(VM)**

```bash
sudo tee /etc/go-todo/gateway.env > /dev/null <<EOF
PORT=8080
GIN_MODE=release
LOG_LEVEL=info
AUTH_SERVICE_URL=http://127.0.0.1:8081
TODO_SERVICE_URL=http://127.0.0.1:8082
ACTIVITY_SERVICE_URL=http://127.0.0.1:8083
JWT_SECRET=$JWT_SECRET
JWT_ISSUER=go-todo-auth
CORS_ORIGINS=http://$SITE
TRUSTED_PROXIES=127.0.0.1
RATE_LIMIT_RPS=10
RATE_LIMIT_BURST=20
EOF
```

What each setting means:

| Variable | Meaning |
|----------|---------|
| `PORT` | Port the service listens on (only reachable inside the VM). |
| `GIN_MODE=release` | No debug output from Gin. |
| `DATABASE_URL` | Each service has its own database + user (database-per-service). |
| `NATS_URL` | Event broker (todo publishes, activity consumes). |
| `JWT_SECRET` | Signs login tokens (auth) and verifies them (gateway). Must be identical in both. |
| `*_SERVICE_URL` | Where the gateway forwards requests. |
| `CORS_ORIGINS` | Website origin allowed to call the API (`https://…` after Part 11). |
| `TRUSTED_PROXIES=127.0.0.1` | Trust nginx's `X-Forwarded-For`, so rate limiting sees the real client IP. |

Lock the files (root can write, the `gotodo` group can read, nobody else can). **(VM)**

```bash
sudo chown root:gotodo /etc/go-todo/*.env
sudo chmod 640 /etc/go-todo/*.env
ls -l /etc/go-todo
```

---

## Part 6 — Postgres + NATS (Docker)

**`/etc/go-todo/postgres-init.sh`**: runs **once**, when the database volume is
empty, and creates one database + user per service. **(VM)**

```bash
sudo tee /etc/go-todo/postgres-init.sh > /dev/null <<'EOF'
#!/bin/sh
# Runs once, when the Postgres volume is empty. Creates one database + one
# user per service, with passwords from /etc/go-todo/infra.env.
# Use hex passwords (openssl rand -hex 24) so no quoting problems can happen.
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<EOSQL
CREATE USER auth_user WITH PASSWORD '${AUTH_DB_PASSWORD}';
CREATE DATABASE auth_db OWNER auth_user;

CREATE USER todo_user WITH PASSWORD '${TODO_DB_PASSWORD}';
CREATE DATABASE todo_db OWNER todo_user;

CREATE USER activity_user WITH PASSWORD '${ACTIVITY_DB_PASSWORD}';
CREATE DATABASE activity_db OWNER activity_user;
EOSQL
EOF
sudo chmod 755 /etc/go-todo/postgres-init.sh
```

**`/etc/go-todo/docker-compose.infra.yml`** **(VM)**

```bash
sudo tee /etc/go-todo/docker-compose.infra.yml > /dev/null <<'EOF'
# Production infrastructure: ONLY Postgres + NATS run in Docker.
# The Go services run natively under supervisord (see supervisor/go-todo.conf).
#
# Lives at: /etc/go-todo/docker-compose.infra.yml (next to postgres-init.sh)
#   sudo docker compose -f /etc/go-todo/docker-compose.infra.yml --env-file /etc/go-todo/infra.env up -d --wait
#
# Ports are bound to 127.0.0.1: Docker-published ports BYPASS ufw, so "0.0.0.0"
# here would expose your database to the whole internet.
name: go-todo-infra

services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set in /etc/go-todo/infra.env}
      # read by postgres-init.sh on the FIRST start (empty volume) only
      AUTH_DB_PASSWORD: ${AUTH_DB_PASSWORD:?set in /etc/go-todo/infra.env}
      TODO_DB_PASSWORD: ${TODO_DB_PASSWORD:?set in /etc/go-todo/infra.env}
      ACTIVITY_DB_PASSWORD: ${ACTIVITY_DB_PASSWORD:?set in /etc/go-todo/infra.env}
    ports:
      - "127.0.0.1:5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./postgres-init.sh:/docker-entrypoint-initdb.d/10-init.sh:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 10

  nats:
    image: nats:2.11-alpine
    restart: unless-stopped
    command: ["-js", "-sd", "/data", "-m", "8222"]
    ports:
      - "127.0.0.1:4222:4222"
      - "127.0.0.1:8222:8222"
    volumes:
      - natsdata:/data
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://127.0.0.1:8222/healthz"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  pgdata:
  natsdata:
EOF
```

Key points of this file:

| Setting | Why |
|---------|-----|
| `127.0.0.1:5432:5432` | Postgres is reachable from the VM only, never from the internet. |
| `${POSTGRES_PASSWORD:?…}` | Read from `infra.env`; Compose refuses to start if it's missing. |
| `./postgres-init.sh:/docker-entrypoint-initdb.d/…` | Postgres runs scripts in that folder on first start. |
| `restart: unless-stopped` | Docker starts them again after a reboot. |
| `volumes: pgdata, natsdata` | Data survives container restarts and upgrades. |

Start them. **(VM)**

```bash
sudo docker compose -f /etc/go-todo/docker-compose.infra.yml --env-file /etc/go-todo/infra.env up -d --wait
sudo docker compose -f /etc/go-todo/docker-compose.infra.yml ps
sudo ss -ltnp | grep -E ':5432|:4222'
```

The last command must show `127.0.0.1:5432` and `127.0.0.1:4222`.

---

## Part 7 — The start wrapper (`/usr/local/bin/go-todo-run`)

supervisord will start each service with `go-todo-run <env file> <binary>`. The
wrapper loads the env file, then **replaces itself** with the binary (`exec`).
That way, supervisord's stop signal (SIGTERM) reaches the Go program directly,
and the graceful shutdown runs. **(VM)**

```bash
sudo tee /usr/local/bin/go-todo-run > /dev/null <<'EOF'
#!/bin/sh
# Lives at: /usr/local/bin/go-todo-run
# Usage: go-todo-run <env-file> <program> [args...]
#
# Loads KEY=value lines from <env-file> into the environment, then REPLACES
# itself with <program> (exec). Because of exec, supervisord's SIGTERM goes
# straight to the Go binary → our graceful shutdown runs.
set -eu
env_file=$1
shift
set -a        # export every variable defined while sourcing
. "$env_file"
set +a
exec "$@"
EOF
sudo chmod 755 /usr/local/bin/go-todo-run
```

---

## Part 8 — Supervisor: the file that starts all the servers

### How supervisor works

| Piece | Role |
|-------|------|
| `systemd` | Starts **supervisord** when the VM boots (`systemctl status supervisor`). |
| **supervisord** | The daemon. Reads every `*.conf` in `/etc/supervisor/conf.d/`, runs each program's `command=`, restarts it if it dies, and writes its logs. |
| **`/etc/supervisor/conf.d/go-todo.conf`** | **Your start commands.** One `[program:…]` block per server. |
| **supervisorctl** | The remote control you type: `status`, `start`, `stop`, `restart`, `tail`, `reread`, `update`. |

### Create `/etc/supervisor/conf.d/go-todo.conf`

**(VM)**

```bash
sudo tee /etc/supervisor/conf.d/go-todo.conf > /dev/null <<'EOF'
; supervisord config for Go-ToDo.
; Lives at: /etc/supervisor/conf.d/go-todo.conf   (see docs/deployment/deployment.md)
;
; supervisord (started by systemd at boot) reads this file and keeps every
; program running: starts it, restarts it if it crashes, rotates its logs.
; supervisorctl is only the remote control (status / restart / tail).

[group:go-todo]
programs=auth-service,todo-service,activity-service,api-gateway

[program:auth-service]
command=/usr/local/bin/go-todo-run /etc/go-todo/auth.env /opt/go-todo/bin/auth-service
directory=/opt/go-todo
user=gotodo
; lower priority number starts first: the gateway starts after the services
priority=100
autostart=true
autorestart=true
; must stay up 3s to count as "started"; retries cover Docker (DB) starting late at boot
startsecs=3
startretries=10
; SIGTERM → graceful shutdown (our httpserver waits up to 10s for in-flight requests)
stopsignal=TERM
stopwaitsecs=20
stopasgroup=true
killasgroup=true
; one log file per program, rotated by supervisord
redirect_stderr=true
stdout_logfile=/var/log/go-todo/auth-service.log
stdout_logfile_maxbytes=20MB
stdout_logfile_backups=5

[program:todo-service]
command=/usr/local/bin/go-todo-run /etc/go-todo/todo.env /opt/go-todo/bin/todo-service
directory=/opt/go-todo
user=gotodo
; lower priority number starts first: the gateway starts after the services
priority=100
autostart=true
autorestart=true
; must stay up 3s to count as "started"; retries cover Docker (DB) starting late at boot
startsecs=3
startretries=10
; SIGTERM → graceful shutdown (our httpserver waits up to 10s for in-flight requests)
stopsignal=TERM
stopwaitsecs=20
stopasgroup=true
killasgroup=true
; one log file per program, rotated by supervisord
redirect_stderr=true
stdout_logfile=/var/log/go-todo/todo-service.log
stdout_logfile_maxbytes=20MB
stdout_logfile_backups=5

[program:activity-service]
command=/usr/local/bin/go-todo-run /etc/go-todo/activity.env /opt/go-todo/bin/activity-service
directory=/opt/go-todo
user=gotodo
; lower priority number starts first: the gateway starts after the services
priority=100
autostart=true
autorestart=true
; must stay up 3s to count as "started"; retries cover Docker (DB) starting late at boot
startsecs=3
startretries=10
; SIGTERM → graceful shutdown (our httpserver waits up to 10s for in-flight requests)
stopsignal=TERM
stopwaitsecs=20
stopasgroup=true
killasgroup=true
; one log file per program, rotated by supervisord
redirect_stderr=true
stdout_logfile=/var/log/go-todo/activity-service.log
stdout_logfile_maxbytes=20MB
stdout_logfile_backups=5

[program:api-gateway]
command=/usr/local/bin/go-todo-run /etc/go-todo/gateway.env /opt/go-todo/bin/api-gateway
directory=/opt/go-todo
user=gotodo
; lower priority number starts first: the gateway starts after the services
priority=200
autostart=true
autorestart=true
; must stay up 3s to count as "started"; retries cover Docker (DB) starting late at boot
startsecs=3
startretries=10
; SIGTERM → graceful shutdown (our httpserver waits up to 10s for in-flight requests)
stopsignal=TERM
stopwaitsecs=20
stopasgroup=true
killasgroup=true
; one log file per program, rotated by supervisord
redirect_stderr=true
stdout_logfile=/var/log/go-todo/api-gateway.log
stdout_logfile_maxbytes=20MB
stdout_logfile_backups=5
EOF
```

> Don't run `supervisorctl update` yet: the binaries don't exist until Part 9
> builds them.

### Every line explained

| Line | Meaning |
|------|---------|
| `[group:go-todo]` + `programs=` | Groups the 4 programs, so `go-todo:*` addresses all of them. |
| `[program:todo-service]` | A program's name (`supervisorctl status` shows `go-todo:todo-service`). |
| **`command=`** | **The start command:** `go-todo-run /etc/go-todo/todo.env /opt/go-todo/bin/todo-service`. |
| `directory=/opt/go-todo` | Working directory. |
| `user=gotodo` | Runs as the unprivileged app user, never root. |
| `priority=100` / `200` | Start order: the 3 services (100) start before the gateway (200). |
| `autostart=true` | Start when supervisord starts, i.e. at boot. |
| `autorestart=true` | Restart automatically after a crash. |
| `startsecs=3` | Must stay up 3 s to count as successfully started. |
| `startretries=10` | Keep retrying, e.g. if Postgres is still starting after a reboot. |
| `stopsignal=TERM` | Stop politely → the Go server finishes in-flight requests (graceful shutdown). |
| `stopwaitsecs=20` | Force-kill if it hasn't exited after 20 s. |
| `stopasgroup` / `killasgroup` | Signal the whole process group (no orphan processes). |
| `redirect_stderr=true` | Errors go into the same log file. |
| `stdout_logfile=/var/log/go-todo/<name>.log` | Log file; rotated at 20 MB, 5 old files kept. |

---

## Part 9 — The deploy script: build + migrate + start

**`/usr/local/bin/go-todo-deploy`** does a complete deploy, as the comments show.
You run it for the first deploy and for every update. **(VM)**

```bash
sudo tee /usr/local/bin/go-todo-deploy > /dev/null <<'EOF'
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
EOF
sudo chmod 755 /usr/local/bin/go-todo-deploy
```

| Step | What happens |
|------|--------------|
| 1 | `git pull` as `gotodo` |
| 2 | `make release` → `bin/api-gateway`, `bin/auth-service`, `bin/todo-service`, `bin/activity-service` (static Go binaries) |
| 3 | `npm ci && npm run build` → `frontend/dist` |
| 4 | Ensures Postgres + NATS are up, then runs every service's SQL migrations (`migrate/migrate` image) |
| 5 | `supervisorctl reread` (reload the config file) → `update` (add new programs) → `restart go-todo:*` → status + health checks |

Run the first deploy (takes a few minutes the first time). **(VM)**

```bash
sudo go-todo-deploy
```

Expected ending:

```
go-todo:activity-service         RUNNING   pid 2345, uptime 0:00:04
go-todo:api-gateway              RUNNING   pid 2346, uptime 0:00:04
go-todo:auth-service             RUNNING   pid 2347, uptime 0:00:04
go-todo:todo-service             RUNNING   pid 2348, uptime 0:00:04
==> health
{"status":"ok"}
{"checks":{"db":"ok"},"status":"ready"}
{"checks":{"db":"ok","nats":"ok"},"status":"ready"}
{"checks":{"db":"ok","nats":"ok"},"status":"ready"}
```

Try the API on the VM (the gateway isn't public, which is correct). **(VM)**

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/auth/register -H 'Content-Type: application/json' -d '{"email":"me@example.com","name":"Me","password":"password123"}' | jq
```

---

## Part 10 — nginx: the public front door

### Create `/etc/nginx/sites-available/go-todo`

**(VM)**

```bash
sudo tee /etc/nginx/sites-available/go-todo > /dev/null <<'EOF'
# nginx site for Go-ToDo on the VM.
# Lives at: /etc/nginx/sites-available/go-todo  (see docs/deployment/deployment.md)
server {
    listen 80;
    listen [::]:80;
    server_name _;                         # replaced with your domain / IP

    root /opt/go-todo/frontend/dist;
    index index.html;
    client_max_body_size 1m;

    # API → api-gateway running under supervisord on localhost
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
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
        expires -1;
        try_files $uri $uri/ /index.html;
    }

    # Security headers (inherited because the locations above use `expires`, not add_header)
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    gzip on;
    gzip_types text/plain text/css application/json application/javascript image/svg+xml;
}
EOF
sudo sed -i "s/server_name _; .*/server_name $SITE;/" /etc/nginx/sites-available/go-todo
grep server_name /etc/nginx/sites-available/go-todo
```

(If your SSH session was closed since Part 5, run `SITE=...` again first.)

### Every part explained

| Part | Meaning |
|------|---------|
| `listen 80;` `listen [::]:80;` | HTTP on IPv4 + IPv6. certbot adds HTTPS (443) in Part 11. |
| `server_name` | Your domain or IP. |
| `root /opt/go-todo/frontend/dist;` | The built React app. |
| `client_max_body_size 1m;` | Reject oversized requests early. |
| **`location /api/ { proxy_pass http://127.0.0.1:8080; }`** | **Every API call goes to the gateway.** No `/` after `8080`, so the path is passed unchanged (`/api/v1/todos` stays `/api/v1/todos`). The gateway then routes to auth / todo / activity. |
| `proxy_set_header X-Forwarded-For …` / `X-Real-IP` | The real client IP, for the gateway's rate limiter and logs. |
| `proxy_set_header X-Forwarded-Proto` | Tells the backend whether the user came via http or https. |
| `location /assets/ { expires 1y; }` | Vite's hashed JS/CSS files are cached for a year. |
| `location / { try_files $uri $uri/ /index.html; }` | **SPA fallback**: `/login` isn't a file, so return `index.html` and let React Router show the page. |
| `expires -1;` | `index.html` is never cached, so users get new versions immediately. |
| `add_header …` | Security headers (no MIME sniffing, no framing, strict referrer). |
| `gzip on;` | Compress text responses. |

### Enable it

**(VM)**

```bash
sudo ln -sf /etc/nginx/sites-available/go-todo /etc/nginx/sites-enabled/go-todo
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl reload nginx
```

Check nginx can read the app, and test locally. **(VM)**

```bash
sudo -u www-data ls /opt/go-todo/frontend/dist
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1/
curl -s http://127.0.0.1/api/v1/todos | jq
```

Expect `200`, then `{"error":{"code":"unauthorized",…}}`. The second one is
correct: you're not logged in, and the request reached the gateway.

Now open **`http://YOUR_DOMAIN_OR_VM_IP`** in your browser, register, and add todos. 🎉

---

## Part 11 — Domain + HTTPS

1. At your DNS provider, create an **A record**: `todo.example.com → VM_IP`.
   Check it from your Mac (wait until it prints the IP): `dig +short todo.example.com`
2. Make sure `server_name todo.example.com;` is set (Part 10).
3. Get a free certificate. certbot edits the nginx file and adds the 443 block
   and an HTTP→HTTPS redirect. **(VM)**

```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot --nginx -d todo.example.com
sudo certbot renew --dry-run
```

4. Switch CORS to https and restart the gateway. **(VM)**

```bash
sudo sed -i 's#^CORS_ORIGINS=http://#CORS_ORIGINS=https://#' /etc/go-todo/gateway.env
grep CORS /etc/go-todo/gateway.env
sudo supervisorctl restart go-todo:api-gateway
```

Open **`https://todo.example.com`** 🔒. Certificates renew automatically (certbot installs a timer).

---

## Part 12 — Everyday commands

| Task | Command |
|------|---------|
| Status of all services | `sudo supervisorctl status` |
| Restart all 4 | `sudo supervisorctl restart 'go-todo:*'` |
| Restart one | `sudo supervisorctl restart go-todo:todo-service` |
| Stop / start one | `sudo supervisorctl stop go-todo:auth-service` · `sudo supervisorctl start go-todo:auth-service` |
| Live log of one | `sudo supervisorctl tail -f go-todo:api-gateway` |
| Log files | `sudo tail -f /var/log/go-todo/todo-service.log` |
| You changed `go-todo.conf` | `sudo supervisorctl reread && sudo supervisorctl update` |
| You changed a `.env` file | `sudo supervisorctl restart go-todo:<service>` |
| Supervisor daemon itself | `sudo systemctl status supervisor` · `sudo systemctl restart supervisor` |
| nginx changed | `sudo nginx -t && sudo systemctl reload nginx` |
| nginx logs | `sudo tail -f /var/log/nginx/access.log /var/log/nginx/error.log` |
| Postgres / NATS status | `sudo docker compose -f /etc/go-todo/docker-compose.infra.yml ps` |
| Database shell | `sudo docker exec -it go-todo-infra-postgres-1 psql -U postgres` |
| Deploy a new version | push from your Mac, then `sudo go-todo-deploy` on the VM |
| Firewall | `sudo ufw status verbose` |

## Part 13 — Reboot test

Everything must come back on its own. **(VM)**

```bash
sudo reboot
```

SSH in again after a minute. **(VM)**

```bash
sudo supervisorctl status
sudo docker ps --format '{{.Names}} {{.Status}}'
curl -s http://127.0.0.1/api/v1/todos | jq -r .error.code
```

All 4 should be `RUNNING`, both containers `Up (healthy)`, and the last line `unauthorized`.

## Part 14 — Troubleshooting

| Symptom | Cause → fix |
|---------|-------------|
| SSH times out | Your home IP changed. AWS: re-run the port-22 `authorize-security-group-ingress` with the new `$MY_IP`. GCP: `gcloud compute firewall-rules update go-todo-allow-ssh --source-ranges=NEW_IP/32`. |
| Browser can't connect | Port 80/443 not open in the cloud firewall (2A.3 / 2B) or ufw (3.4); `sudo systemctl status nginx`. |
| nginx shows **502 Bad Gateway** | The gateway isn't running: `sudo supervisorctl status go-todo:api-gateway`. |
| JSON `{"code":"bad_gateway"}` | The gateway is fine, but a service behind it is down: `sudo supervisorctl status`. |
| A service is `FATAL` / `BACKOFF` | `sudo supervisorctl tail go-todo:<name>`: usually an `.env` problem (`JWT_SECRET is not set`, `password authentication failed`). |
| `spawn error` | No binary (`ls /opt/go-todo/bin` → run `sudo go-todo-deploy`) or the wrapper isn't executable (`sudo chmod 755 /usr/local/bin/go-todo-run`). |
| `permission denied` for an `.env` | `sudo chown root:gotodo /etc/go-todo/*.env && sudo chmod 640 /etc/go-todo/*.env` |
| You log in, then everything is 401 | `JWT_SECRET` differs between `auth.env` and `gateway.env`. |
| `password authentication failed` | `infra.env` changed after Postgres was first created. Fix the passwords in the `.env` files, or (deletes all data) `sudo docker compose -f /etc/go-todo/docker-compose.infra.yml down -v` and start again. |
| Build `signal: killed` | Out of memory → check swap (3.2): `free -h`. |
| certbot fails | DNS doesn't point to the VM yet, or port 80 is closed. |
| Old app after deploy | Hard refresh (Ctrl+Shift+R). |

---

## Part 15 — Delete everything

### AWS **(Mac)**

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

Closed the terminal and lost the IDs? Find them by name. **(Mac)**

```bash
aws ec2 describe-vpcs --filters Name=tag:Name,Values=go-todo-vpc --query 'Vpcs[].VpcId' --output text
aws ec2 describe-instances --filters Name=tag:Name,Values=go-todo --query 'Reservations[].Instances[].InstanceId' --output text
aws ec2 describe-addresses --query 'Addresses[].[AllocationId,PublicIp]' --output text
```

### GCP **(Mac)**

```bash
gcloud compute instances delete go-todo --zone=asia-south1-a --quiet
gcloud compute addresses delete go-todo-ip --region=asia-south1 --quiet
gcloud compute firewall-rules delete go-todo-allow-ssh go-todo-allow-web --quiet
gcloud compute networks subnets delete go-todo-public --region=asia-south1 --quiet
gcloud compute networks delete go-todo-vpc --quiet
```

---

## Next steps

- Managed database (RDS / Cloud SQL) with automatic backups.
- VM in a private subnet behind a load balancer (ALB / Cloud Load Balancing) + NAT gateway.
- Build binaries in GitHub Actions and copy them to the VM instead of compiling on it.
- Replace supervisord with systemd units, or move to Kubernetes (EKS / GKE).

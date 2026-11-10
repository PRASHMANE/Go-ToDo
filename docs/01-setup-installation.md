# 01 — Setup & Installation (macOS)

Install everything once, then verify each tool. If a verify command fails,
fix it before moving on.

## 1. Homebrew (package manager)

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
brew --version
```

## 2. Go

```bash
brew install go
go version            # expect go1.25 or newer
go env GOPATH         # usually /Users/<you>/go
go env GOMODCACHE     # where downloaded modules live
```

Add Go-installed tools to your PATH (add to `~/.zshrc`, then `source ~/.zshrc`):

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

**What you just installed:**

| Command | What it does |
|---------|--------------|
| `go run ./cmd` | Compile + run a main package (temporary binary) |
| `go build ./...` | Compile every package (catches errors) |
| `go test ./...` | Run all tests |
| `go vet ./...` | Static checks for common mistakes |
| `go mod init <path>` | Create a new module (`go.mod`) |
| `go get <pkg>@<ver>` | Add/upgrade a dependency |
| `go mod tidy` | Add missing / remove unused deps, update `go.sum` |
| `go work init / use` | Create / extend a multi-module workspace |
| `go fmt ./...` / `gofmt -w .` | Format code (Go has ONE official style) |
| `go doc net/http.Handler` | Read docs in the terminal |

## 3. Go developer tools

```bash
# Hot reload: rebuilds & restarts your service when you save a file
go install github.com/air-verse/air@latest

# Database migrations CLI (with Postgres support)
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Linter aggregator (runs many linters at once)
brew install golangci-lint

# Verify
air -v
migrate -version
golangci-lint --version
```

## 4. Node.js (for the React frontend)

Use **nvm** so you can switch Node versions per project:

```bash
brew install nvm
mkdir -p ~/.nvm
# add these 2 lines to ~/.zshrc, then: source ~/.zshrc
export NVM_DIR="$HOME/.nvm"
[ -s "$(brew --prefix nvm)/nvm.sh" ] && . "$(brew --prefix nvm)/nvm.sh"

nvm install 22
nvm alias default 22
node -v    # v22.x
npm -v
```

## 5. Docker Desktop

```bash
brew install --cask docker
open -a Docker                  # start it once; wait for the whale icon
docker version
docker compose version          # Compose v2 (command is "docker compose", with a space)
docker run --rm hello-world
```

## 6. Database & HTTP helper tools

```bash
brew install libpq && brew link --force libpq   # gives you `psql` without installing a server
brew install jq                                  # pretty-print JSON in the terminal
brew install httpie                              # friendlier curl (optional)
brew install nats-io/nats-tools/nats             # NATS CLI (used in Phase B8)

psql --version
jq --version
```

`curl` is already on macOS. Examples in the docs use `curl ... | jq`.

## 7. Editor: VS Code extensions

```bash
code --install-extension golang.go                 # Go: IntelliSense, debug, gopls
code --install-extension dbaeumer.vscode-eslint
code --install-extension esbenp.prettier-vscode
code --install-extension bradlc.vscode-tailwindcss
code --install-extension ms-azuretools.vscode-docker
code --install-extension humao.rest-client         # send HTTP requests from .http files
```

After installing the Go extension, open the command palette → **"Go: Install/Update Tools"**
→ select all → OK. This installs `gopls` (language server) and `dlv` (debugger).

Recommended VS Code `settings.json`:

```json
{
  "go.lintTool": "golangci-lint",
  "[go]": { "editor.formatOnSave": true, "editor.codeActionsOnSave": { "source.organizeImports": "explicit" } },
  "editor.formatOnSave": true
}
```

## 8. Git identity for THIS repo only

Already done for this project, but this is how you set and check it:

```bash
cd ~/Documents/Go_ToDo_Project
git config user.name  "prashanth v"
git config user.email "prashanthvprashanthv46@gmail.com"
git config user.name && git config user.email   # verify
```

(No `--global`, so your work identity stays the default everywhere else.)

## 9. Final checklist

```bash
go version && air -v && migrate -version && golangci-lint --version \
 && node -v && npm -v && docker compose version && psql --version && jq --version
```

All of these print a version → you're ready for
[Backend Phase 0](backend/phase-0-foundations-and-repo-setup.md).

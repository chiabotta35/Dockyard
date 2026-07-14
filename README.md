# Dockyard

Docker container update manager with a web UI, forked from [watchtower](https://github.com/nicholas-fedor/watchtower).

Dockyard monitors running Docker containers and updates them when new images are released. It features a web dashboard for managing updates, per-container update modes, deferred updates, and self-updating.

## Features

- **Web Dashboard** -- Dark-themed UI with real-time SSE log streaming
- **Per-Container Modes** -- Set each container to auto, manual, or ignore
- **Deferred Updates** -- Postpone updates for specific containers (7, 14, 30+ days)
- **Self-Update** -- Update Dockyard itself from the web UI
- **Authentication** -- bcrypt password hashing, session-based auth, CSRF-safe cookies
- **Scheduled Updates** -- Cron-based scheduling (default: daily at 3 AM)
- **SSE Live Logs** -- Real-time event streaming for container operations
- **Update History** -- Track all past updates with timestamps and status
- **Settings** -- Configure schedule, behavior, backup, and notifications from the UI

## Installation

### Prerequisites

- Docker and Docker Compose installed on the target machine
- Access to the Docker socket (`/var/run/docker.sock`)

### Step 1: Clone the repository

```bash
git clone https://github.com/chiabotta35/Dockyard.git
cd Dockyard
```

### Step 2: Build and start

```bash
docker compose up -d --build
```

This builds the image and starts Dockyard on port **8082**.

### Step 3: Open the web UI

Navigate to `http://<your-server-ip>:8080` in your browser. On first launch you will be prompted to create an admin account.

### Step 4: Stop watchtower (if running)

Dockyard replaces watchtower. If you have watchtower running, stop and remove it:

```bash
docker stop watchtower && docker rm watchtower
```

### Step 5: Configure via proxy (optional)

If you use nginx-proxy-manager or another reverse proxy, add a new proxy host for Dockyard:

| Field | Value |
|-------|-------|
| Domain | e.g. `dockyard.yourdomain.com` |
| Forward Hostname / IP | `your-server-ip` |
| Forward Port | `8082` |
| WebSocket Support | Enabled |

## Docker Compose

The default `docker-compose.yml` maps Dockyard to port **8082** on localhost:

```yaml
services:
  dockyard:
    build: .
    container_name: dockyard
    restart: unless-stopped
    ports:
      - "127.0.0.1:8082:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./data:/app/data
    environment:
      - DOCKER_HOST=unix:///var/run/docker.sock
      - TZ=UTC
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: "1.0"
```

To change the port, edit the `ports` mapping:

```yaml
ports:
  - "9090:8080"   # external port : internal port
```

To expose on all interfaces (not recommended without auth):

```yaml
ports:
  - "8082:8080"
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--web-ui` | `false` | Enable the web dashboard |
| `--web-ui-host` | `""` (all) | Bind host for the web UI |
| `--web-ui-port` | `8080` | Port for the web UI |
| `--web-ui-data` | `/app/data` | Data directory for state persistence |
| `--schedule` | `0 3 * * *` | Cron schedule for updates |
| `--cleanup` | `true` | Remove old images after update |
| `--run-once` | `false` | Check once and exit |
| `--monitor-only` | `false` | Monitor without updating |
| `--no-restart` | `false` | Skip container restart after update |
| `--rolling-restart` | `false` | Update containers one at a time |

All flags can also be set via environment variables with a `WATCHTOWER_` prefix.

## Architecture

```
cmd/root.go              CLI entry point, flag parsing, orchestration
internal/webui/
  server.go              HTTP server, routing, security middleware
  handlers.go            Page and API handlers
  state.go               JSON state persistence (containers, settings, history)
  events.go              SSE event hub with history buffer
  auth.go                Authentication, sessions, bcrypt
  updater.go             Self-update via GitHub releases
internal/actions/        Container update logic
internal/scheduling/     Cron scheduler
pkg/container/           Docker client wrapper
pkg/types/               Shared types and interfaces
```

## Security

- **Authentication**: bcrypt password hashing, 32-byte random session tokens
- **Cookies**: HttpOnly, SameSite=Strict, configurable Secure flag
- **Sessions**: Invalidate all sessions on password change
- **Headers**: CSP, X-Frame-Options: DENY, X-Content-Type-Options: nosniff, X-XSS-Protection
- **Input Validation**: Container name sanitization, request body size limits (1 MB), URL scheme validation
- **File Permissions**: Auth and state files written with `0600`
- **Self-Update**: Direct HTTP download (no shell execution), SHA-256 checksum logging, backup/rollback on failure
- **Docker**: Non-root user in container, read-only root filesystem, `no-new-privileges`, resource limits

## Environment Variables

| Variable | Description |
|----------|-------------|
| `DOCKER_HOST` | Docker daemon socket |
| `DOCKER_TLS_VERIFY` | Enable TLS for Docker |
| `DOCKER_API_VERSION` | Docker API version |
| `WATCHTOWER_SCHEDULE` | Cron schedule |
| `WATCHTOWER_CLEANUP` | Remove old images |
| `WATCHTOWER_MONITOR_ONLY` | Monitor without updating |
| `WATCHTOWER_LOG_LEVEL` | Log level (debug, info, warn, error) |

## License

Apache License 2.0 -- see [LICENSE](LICENSE) for details.

Originally based on [watchtower](https://github.com/nicholas-fedor/watchtower) by Nicholas Fedor.

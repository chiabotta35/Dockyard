# Dockyard

Docker container update manager with a web UI, forked from [watchtower](https://github.com/nicholas-fedor/watchtower).

Monitors running Docker containers and updates them when new images are released. Features a web dashboard for managing updates, per-container update modes, deferred updates, and self-updating.

## Install

Create a `docker-compose.yml` on your server:

```yaml
services:
  dockyard:
    image: ghcr.io/chiabotta35/dockyard:latest
    container_name: dockyard
    restart: unless-stopped
    ports:
      - "127.0.0.1:8082:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - dockyard-data:/app/data
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

volumes:
  dockyard-data:
```

Then run:

```bash
docker compose up -d
```

Open `http://<your-server-ip>:8082` and create your admin account.

### Stopping watchtower

Dockyard replaces watchtower. If you have it running:

```bash
docker stop watchtower && docker rm watchtower
```

### Changing the port

Edit the `ports` line in the compose file:

```yaml
ports:
  - "9090:8080"   # your-port:container-port
```

### Building from source

```bash
git clone https://github.com/chiabotta35/Dockyard.git
cd Dockyard
docker compose up -d --build
```

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

## Security

- **Authentication**: bcrypt password hashing, 32-byte random session tokens
- **Cookies**: HttpOnly, SameSite=Strict, configurable Secure flag
- **Sessions**: Invalidate all sessions on password change
- **Headers**: CSP, X-Frame-Options: DENY, X-Content-Type-Options: nosniff, X-XSS-Protection
- **Input Validation**: Container name sanitization, request body size limits (1 MB), URL scheme validation
- **File Permissions**: Auth and state files written with `0600`
- **Self-Update**: Direct HTTP download (no shell execution), SHA-256 checksum logging, backup/rollback on failure
- **Docker**: Non-root user in container, read-only root filesystem, `no-new-privileges`, resource limits

## License

Apache License 2.0 -- see [LICENSE](LICENSE) for details.

Originally based on [watchtower](https://github.com/nicholas-fedor/watchtower) by Nicholas Fedor.

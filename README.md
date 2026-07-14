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
      - "8082:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./data:/app/data
    environment:
      - DOCKYARD_ADMIN_USER=admin
      - DOCKYARD_ADMIN_PASSWORD=changeme
      - DOCKYARD_SCHEDULE=0 3 * * *
      - DOCKYARD_CLEANUP=true
      - DOCKER_HOST=unix:///var/run/docker.sock
      - TZ=UTC
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: "1.0"
```

Then:

```bash
docker compose up -d
```

Open `http://<your-server-ip>:8082` and sign in with the credentials from `DOCKYARD_ADMIN_USER` / `DOCKYARD_ADMIN_PASSWORD`.

### Stopping watchtower

Dockyard replaces watchtower. If you have it running:

```bash
docker stop watchtower && docker rm watchtower
```

### Changing the port

Edit the `ports` line:

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

## Environment Variables

All settings below can be set in your `docker-compose.yml` and are also editable in the web UI after first launch.

### Admin Account

| Variable | Required | Description |
|----------|----------|-------------|
| `DOCKYARD_ADMIN_USER` | Yes | Admin username |
| `DOCKYARD_ADMIN_PASSWORD` | Yes | Admin password (min 8 chars). Changing this resets the password for the user. |

### Schedule

| Variable | Default | Description |
|----------|---------|-------------|
| `DOCKYARD_SCHEDULE` | `0 3 * * *` | Cron schedule for update checks (use the web UI for friendly dropdown) |
| `DOCKYARD_TIMEZONE` | `UTC` | IANA timezone (e.g. `America/New_York`, `Europe/London`) |
| `DOCKYARD_COOLDOWN_DELAY` | `0s` | Minimum image age before update (e.g. `24h`, `3d`, `1w`) |
| `DOCKYARD_STOP_TIMEOUT` | `30s` | Timeout when stopping containers |

### Backup

| Variable | Default | Description |
|----------|---------|-------------|
| `DOCKYARD_BACKUP_RETENTION` | `false` | Keep old containers for rollback |
| `DOCKYARD_BACKUP_WINDOW_HOURS` | `24` | How long to keep backups (1–168 hours) |

### Notifications

| Variable | Default | Description |
|----------|---------|-------------|
| `DOCKYARD_NOTIFICATION_URL` | (empty) | [Shoutrrr](https://containrrr.dev/shoutrrr/) URL for notifications |

Uses [Shoutrrr](https://containrrr.dev/shoutrrr/) for notifications. Set `DOCKYARD_NOTIFICATION_URL` in your compose file or enter it in **Settings > Notifications** in the web UI.

## Notification Setup

Dockyard sends notifications when container updates start, complete, or fail. Notifications are powered by [Shoutrrr](https://containrrr.dev/shoutrrr/) and work with all major services. Set the URL in your compose file or in **Settings > Notifications** in the web UI.

### ntfy (Easiest — no account needed)

1. Pick a topic name (anything you want — it's created on first use)
2. Subscribe on your phone:
   - **Android**: Install [ntfy](https://play.google.com/store/apps/details?id=io.heckel.ntfy) from the Play Store, tap **Add topic**, enter your topic name
   - **iOS**: Install [ntfy](https://apps.apple.com/us/app/ntfy/id1625396347) from the App Store, tap **Add topic**, enter your topic name
   - **Web**: Open `https://ntfy.sh` in a browser, type your topic name in the subscribe box
3. Set this in your compose file:

```yaml
environment:
  - DOCKYARD_NOTIFICATION_URL=ntfy://dockyard-updates
```

That's it. Notifications will appear on your phone instantly. Works over the free ntfy.sh server — no account, no API key. You can also [self-host ntfy](https://docs.ntfy.sh/install/) and use `ntfy://your-server/topic` instead.

### Discord

1. Open your Discord server
2. Go to **Server Settings > Integrations > Webhooks > New Webhook**
3. Name it (e.g. "Dockyard"), pick the channel, click **Copy Webhook URL**
4. The URL looks like `https://discord.com/api/webhooks/1234567890/abcdefg...`
5. Extract the two parts after `/webhooks/`:

```
https://discord.com/api/webhooks/1234567890/abcdefgHIJKlmnop
                                  ^^^^^^^^^^^  ^^^^^^^^^^^^^
                                  webhook ID   webhook token
```

6. Set this in your compose file:

```yaml
environment:
  - DOCKYARD_NOTIFICATION_URL=discord://1234567890/abcdefgHIJKlmnop
```

### Slack

1. Go to `https://api.slack.com/apps` and click **Create New App > From scratch**
2. Name it "Dockyard", pick your workspace
3. Go to **Incoming Webhooks** and toggle it on
4. Click **Add New Webhook to Workspace**, pick a channel, click **Allow**
5. Copy the webhook URL — it contains three tokens separated by `/`
6. Set this in your compose file:

```yaml
environment:
  # Replace these with your actual Slack webhook tokens
  - DOCKYARD_NOTIFICATION_URL=slack://your-token-a/your-token-b/your-token-c
```

To get the tokens: extract the three strings after `services/` in your webhook URL (e.g. the URL contains `services/T123/B456/xyz789` → tokens are `T123`, `B456`, `xyz789`).

### Telegram

1. Open Telegram, search for `@BotFather`
2. Send `/newbot`, follow the prompts to name your bot
3. BotFather gives you a token like `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`
4. Send a message to your bot (open it and tap **Start**)
5. Get your chat ID by visiting `https://api.telegram.org/bot<YOUR_TOKEN>/getUpdates` — look for `"chat":{"id":` in the response (it's a number like `-1001234567890`)
6. Set this in your compose file:

```yaml
environment:
  - DOCKYARD_NOTIFICATION_URL=telegram://123456789:ABCdefGHIjklMNOpqrsTUVwxyz/-1001234567890
```

### Email (SMTP)

Use any SMTP provider — Gmail, Outlook, self-hosted, etc.

**Gmail example:**
1. Use an [App Password](https://support.google.com/accounts/answer/185833) (not your regular password)
2. Set:

```yaml
environment:
  - DOCKYARD_NOTIFICATION_URL=email://user:app-password@gmail.com/?subject=Dockyard&to=you@email.com&from=you@email.com&host=smtp.gmail.com&port=587&starttls=yes
```

**Generic SMTP:**
```yaml
environment:
  - DOCKYARD_NOTIFICATION_URL=email://user:password@smtp.example.com/?subject=Dockyard&to=you@email.com&host=smtp.example.com&port=587&starttls=yes
```

### Gotify

1. Install [Gotify](https://gotify.net/docs/install) on your server
2. Go to **Apps > Create Application**, name it "Dockyard"
3. Copy the token
4. Set:

```yaml
environment:
  - DOCKYARD_NOTIFICATION_URL=gotify://gotify.example.com/token
```

### Microsoft Teams

1. In Teams, go to a channel, click **...** next to the channel name > **Connectors**
2. Find **Incoming Webhook**, click **Configure**
3. Name it "Dockyard", optionally upload an icon, click **Create**
4. Copy the webhook URL
5. Set:

```yaml
environment:
  - DOCKYARD_NOTIFICATION_URL=teams://webhook-url
```

### Testing

After setting up, click **Test** in Settings > Notifications to send a test message. If it doesn't arrive, check your compose logs:

```bash
docker compose logs dockyard | grep -i notif
```

### Common Issues

- **"unsupported protocol"**: Check the URL scheme matches the service (e.g. `ntfy://`, `discord://`, `slack://`)
- **No message received**: Check the webhook URL / token is correct. For ntfy, make sure your phone is subscribed to the same topic
- **Connection refused**: Make sure Dockyard's container can reach the internet (check your firewall/DNS settings)

### Docker

| Variable | Default | Description |
|----------|---------|-------------|
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon socket |
| `DOCKER_TLS_VERIFY` | (empty) | Enable TLS for Docker connection |
| `DOCKER_API_VERSION` | (empty) | Docker API version override |
| `TZ` | `UTC` | Container timezone |

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

## Security

- **Authentication**: bcrypt password hashing, 32-byte random session tokens
- **Cookies**: HttpOnly, SameSite=Strict, configurable Secure flag
- **Sessions**: Invalidate all sessions on password change
- **Headers**: CSP, X-Frame-Options: DENY, X-Content-Type-Options: nosniff, X-XSS-Protection
- **Input Validation**: Container name sanitization, request body size limits (1 MB), URL scheme validation
- **File Permissions**: Auth and state files written with `0600`
- **Self-Update**: Direct HTTP download (no shell execution), SHA-256 checksum logging, backup/rollback on failure
- **Docker**: Runs as root (required to manage Docker socket — like Watchtower/Portainer). Secure your compose file with `no-new-privileges` and resource limits.

## License

Apache License 2.0 -- see [LICENSE](LICENSE) for details.

Originally based on [watchtower](https://github.com/nicholas-fedor/watchtower) by Nicholas Fedor.

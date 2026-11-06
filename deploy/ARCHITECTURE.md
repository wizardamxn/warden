# Deploy

- `compose.yaml`: Postgres for local development, in Docker.
- `phone/`, `web/`: going live, not built yet.

## Run it locally

Needs Docker Desktop running, and this line in `server/.env`:

```
DATABASE_URL=postgres://warden:warden@127.0.0.1:55432/warden
```

From the repo root, in PowerShell:

```
.\scripts\dev.ps1
```

That starts Postgres, applies the migrations, then runs the API and the worker with their logs in the terminal. Ctrl+C stops the API and the worker. Postgres keeps running until `docker compose -f deploy/compose.yaml stop`.

- Port 55432 stays clear of a normal PostgreSQL install on 5432. Docker publishes it to this computer only.
- The data lives in a Docker volume. To start empty: `docker compose -f deploy/compose.yaml down -v`.

## Before going public

The API has no login yet. It binds to 127.0.0.1 by default and must not be put behind the tunnel, or exposed any other way, until step 6 (accounts and limits) is built.

# Heimly Backend

## Self-Hosted (Docker Compose) 🚀

No public Docker image yet, so Compose builds backend locally from this repo 👇

### 1) Prep env 🛠️

```bash
cp .env.example .env
```

Then open `.env` and set real secrets/passwords.

### 2) Start everything ▶️

```bash
docker compose up -d --build
```

### 3) Stop ⏹️

```bash
docker compose down
```

### 4) Logs 👀

```bash
docker compose logs -f
```

## Dev Mode (IDE + local infra) 🧪

### 1) Prepare dev env

```bash
cp .env.dev.example .env.dev
```

### 2) Start only infra

```bash
docker compose -f docker-compose.dev.yml --env-file .env.dev up -d
```

Then run backend from GoLand with environment file `.env.dev`.

Optional: start Scalar docs too:

```bash
docker compose -f docker-compose.dev.yml --env-file .env.dev --profile docs up -d
```

Docs URL: `http://localhost:8081` (or custom `DOCS_PORT`)

### 3) Stop infra

```bash
docker compose -f docker-compose.dev.yml --env-file .env.dev down
```

## API Docs + GoLand HTTP 🔎

- OpenAPI spec: `api/openapi.yaml`
- GoLand HTTP samples: `api/http/health.http`, `api/http/auth.http`
- GoLand HTTP environments: `api/http/http-client.env.json` (`local` / `docker`)
- Scalar docs service: `api-docs` (dev compose profile: `docs`)

In GoLand HTTP client, pick environment (`local` or `docker`) and run requests from `.http` files.

## `.env` Quick Guide 📦

- `HEIMLY_PORT`: Backend port (host + container).
- `SHARE_DATA`: App analytics flag (`true` / `false`). Default is `true`.
- `ACCESS_TOKEN_TTL`: Access token lifetime (Go duration, e.g. `15m`).
- `REFRESH_TOKEN_TTL`: Refresh token lifetime (Go duration, e.g. `720h`).
- `POSTGRES_USER`: Postgres user.
- `POSTGRES_PASSWORD`: Postgres password.
- `POSTGRES_DB`: Postgres database name.
- `POSTGRES_PORT`: Host port for Postgres (`5432` inside container).
- `VALKEY_PORT`: Host port for Valkey (`6379` inside container).
- `RUSTFS_ACCESS_KEY`: RustFS access key.
- `RUSTFS_SECRET_KEY`: RustFS secret key.
- `RUSTFS_PORT`: Host port for RustFS API (`9000` inside container).
- `RUSTFS_CONSOLE_PORT`: Host port for RustFS console (`9001` inside container).
- `NETWORK_NAME`: Docker network name.

## Notes 📝

- Backend config is persisted in volume `heimly_config`.
- Infra data is persisted in `heimly_db`, `heimly_cache`, `heimly_storage`.
- Backend connects internally via Compose service names: `postgres`, `valkey`, `rustfs`.
- Refresh tokens are JWTs; Valkey stores only active refresh `jti` values with TTL and atomic rotation.
- Access tokens include JWT `jti`; middleware validates token is active in Valkey.
- Privacy: we do not collect personal data. `SHARE_DATA` only controls aggregated app usage stats/analytics.

## GoLand Docker Debugging 🐞

Yes, you can debug via Docker in GoLand.

- Easiest flow: run infra with `docker-compose.dev.yml`, run backend in GoLand using `.env.dev`, debug normally.
- Full Docker-debug flow is also possible via Delve inside container + GoLand "Go Remote" config.
- Current repo does not include a dedicated Delve debug Dockerfile yet, so you’d add one when you want full in-container debugging.

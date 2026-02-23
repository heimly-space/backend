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

## `.env` Quick Guide 📦

- `HEIMLY_PORT`: Backend port (host + container).
- `SHARE_DATA`: App analytics flag (`true` / `false`). Default is `true`.
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
- Privacy: we do not collect personal data. `SHARE_DATA` only controls aggregated app usage stats/analytics.

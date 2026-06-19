# SNS — Simple Notification Service

A production-ready email notification service built with Go. Accepts email send requests via HTTP, dispatches them through Kafka, and delivers via SMTP.

## Architecture

```
HTTP Client
    │
    ▼
POST /api/email/raw/send  (JWT authenticated)
    │
    ▼
┌─────────────────┐
│   HTTP Server   │  :8080
│  (chi router)   │
└────────┬────────┘
         │ Go channel (buffered)
         ▼
┌─────────────────┐
│   Dispatcher    │  publishes to Kafka
└────────┬────────┘
         │ topic: email.send
         ▼
┌─────────────────┐     ┌──────────────────────┐
│  Email Worker   │────►│  SMTP Server (Gmail, │
│ (Kafka consumer)│     │   Postfix, etc.)     │
└────────┬────────┘     └──────────────────────┘
         │ topic: email.result
         ▼
    Result published
```

**Components:**
- **HTTP Server** — receives send requests, validates JWT, enqueues to internal channel
- **Dispatcher** — reads from the channel, publishes to Kafka `email.send`
- **Email Worker** — consumes `email.send`, looks up SMTP config by `from` address, sends via SMTP, publishes result to `email.result`
- **Metrics Server** — exposes Prometheus metrics at `:10254/metrics`
- **Admin UI** — single-page app at `/admin` for managing SMTP configs and sending test emails

## Requirements

- Go 1.22+
- Docker & Docker Compose
- Python 3 + [PyJWT](https://pypi.org/project/PyJWT/) (`pip install pyjwt`) — for JWT token generation via `make jwt-token`

## Quick Start

```bash
# 1. Clone and enter the project
git clone https://github.com/benakaben10/sns
cd sns

# 2. Start the full stack (Postgres + Redpanda + app)
docker compose up -d

# 3. Run database migration
make migrate-up

# 4. Create Kafka topics
make topic-send topic-result

# 5. Verify the service is running
curl http://localhost:8080/healthz
# → {"status":"ok"}
```

## Configuration

All configuration is via environment variables. See [`application-config/example.env`](application-config/example.env) for the full list.

| Variable | Default | Description |
|---|---|---|
| `HTTP_PORT` | `8080` | HTTP server port |
| `POSTGRES_DSN` | — | PostgreSQL connection string |
| `KAFKA_BROKERS` | — | Comma-separated broker addresses |
| `KAFKA_EMAIL_SEND_TOPIC` | `email.send` | Topic for outbound email jobs |
| `KAFKA_EMAIL_RESULT_TOPIC` | `email.result` | Topic for delivery results |
| `KAFKA_CONSUMER_GROUP` | `notification-service` | Kafka consumer group ID |
| `CHANNEL_BUFFER_SIZE` | `1000` | Internal Go channel buffer size |
| `JWT_AUTH_MODE` | `symmetric` | `symmetric` or `jwks` |
| `JWT_HMAC_SECRET` | — | Required when `JWT_AUTH_MODE=symmetric` |
| `JWT_JWKS_URL` | — | Required when `JWT_AUTH_MODE=jwks` |
| `JWT_ISSUER` | — | Expected JWT issuer claim |
| `JWT_AUDIENCE` | — | Expected JWT audience claim |

## API

All `/api/*` routes require a valid JWT in the `Authorization: Bearer <token>` header.

### Send Email

```
POST /api/email/raw/send
```

Required scope/role: `email:send` scope **or** `email_sender` realm role **or** `admin` realm role.

**Request:**
```json
{
  "from": "sender@example.com",
  "to": ["recipient@example.com"],
  "title": "Hello",
  "body": "Email body text"
}
```

**Response `202 Accepted`:**
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "queued"
}
```

### SMTP Config CRUD

```
GET    /api/smtp-configs
POST   /api/smtp-configs
GET    /api/smtp-configs/{id}
PUT    /api/smtp-configs/{id}
DELETE /api/smtp-configs/{id}
PATCH  /api/smtp-configs/{id}/default
```

Required scope/role: `smtp:manage` scope **or** `admin` realm role.

**SMTP Config fields:**

| Field | Type | Description |
|---|---|---|
| `name` | string | Display name |
| `host` | string | SMTP host |
| `port` | int | SMTP port (25 / 465 / 587) |
| `username` | string | SMTP username |
| `password` | string | SMTP password (write-only, never returned) |
| `from_email` | string | Match incoming `from` address; empty = default config |
| `use_tls` | bool | Use TLS from the start (port 465) |
| `use_starttls` | bool | Upgrade to TLS via STARTTLS (port 587) |
| `is_default` | bool | Fallback config when no `from_email` match |

### Other Endpoints

| Endpoint | Description |
|---|---|
| `GET /healthz` | Liveness probe — returns `{"status":"ok"}` |
| `GET /admin` | Admin UI (single-page app) |
| `GET /metrics` | Prometheus metrics (port `10254`) |

## Admin UI

Open [http://localhost:8080/admin](http://localhost:8080/admin), paste a valid JWT token to log in.

**SMTP Configs tab** — full CRUD for SMTP configurations.

**Send Test Email tab** — send a test email directly through the queue and see the `request_id` response.

## Development

### Generate a JWT for testing

```bash
# email:send scope
make jwt-token JWT_SCOPE="email:send" JWT_SUB="alice"

# smtp:manage scope (admin)
make jwt-token JWT_SCOPE="smtp:manage" JWT_SUB="admin"

# Store in a variable
TOKEN=$(make jwt-token JWT_SCOPE="email:send smtp:manage" -s)
```

### Send a test email via CLI

```bash
make send-test \
  SEND_FROM=sender@gmail.com \
  SEND_TO=recipient@example.com \
  SEND_TITLE="Hello" \
  SEND_BODY="Test from SNS"
```

### Create an SMTP config via CLI

```bash
make smtp-create \
  SMTP_USER=sender@gmail.com \
  SMTP_PASS="app-password-here" \
  SMTP_FROM=sender@gmail.com \
  SMTP_NAME="Gmail" \
  SMTP_STARTTLS=true
```

> **Gmail note:** Regular passwords are rejected. Generate an [App Password](https://myaccount.google.com/apppasswords) (requires 2FA enabled).

### Make targets

```
Build & Run
  make build           Compile binary to ./bin/
  make run             Run service locally (reads .env if present)
  make clean           Remove build artifacts and test cache

Testing & Code Quality
  make test            Run all unit tests
  make test-verbose    Run tests with verbose output
  make test-race       Run tests with race detector
  make lint            Run golangci-lint
  make fmt             Format all Go code
  make vet             Run go vet
  make tidy            Run go mod tidy

Docker
  make docker-build    Build Docker image from deploy/Dockerfile
  make up              Start full stack with docker compose
  make down            Stop and remove containers (volumes preserved)
  make restart         Restart only the app container
  make logs-app        Tail app logs
  make logs-db         Tail postgres logs
  make logs-kafka      Tail redpanda logs

Database
  make migrate-up      Apply schema migration (creates smtp_configs table)
  make migrate-seed    Insert seed SMTP config
  make psql            Open psql shell in the postgres container

Kafka
  make topic-send      Create email.send topic
  make topic-result    Create email.result topic
  make consume-send    Watch email.send topic in real time
  make consume-result  Watch email.result topic in real time

Utilities
  make jwt-token       Generate a signed JWT (requires python3 + pyjwt)
  make smtp-create     Create an SMTP config via API
  make send-test       Send a test email via API
```

## Project Structure

```
sns/
├── cmd/notification-service/ ← main entry point
├── internal/
│   ├── auth/              ← JWT middleware, claims, HMAC/JWKS verifiers
│   ├── channel/           ← internal Go channel dispatcher
│   ├── config/            ← environment variable loading
│   ├── http/
│   │   ├── handler/       ← email and SMTP config HTTP handlers
│   │   ├── response/      ← shared JSON response helpers
│   │   └── router/        ← chi router wiring + embedded admin UI
│   ├── model/             ← shared data types
│   ├── queue/             ← Kafka producer and consumer
│   ├── repository/        ← PostgreSQL SMTP config repository
│   ├── smtp/              ← SMTP client (plain / TLS / STARTTLS)
│   └── worker/            ← Kafka consumer → SMTP delivery worker
├── deploy/
│   ├── Dockerfile         ← multi-stage build (FLI Platform compliant)
│   ├── entrypoint.sh      ← platform entrypoint (do not modify)
│   └── entrypoints/
│       └── docker-entrypoint.sh
├── application-config/
│   ├── example.env        ← environment variable template
│   └── example.config.json
├── migrations/
│   ├── 001_create_smtp_configs.sql
│   └── 002_seed_smtp_config.sql
├── docker-compose.yml     ← local dev stack
└── Makefile
```

## License

MIT

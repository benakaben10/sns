# ─────────────────────────────────────────────────────────────────────────────
#  Simple Notification Service (SNS) — Makefile
#  Usage: make <target>
#         make help   (show this list)
# ─────────────────────────────────────────────────────────────────────────────

# ── Variables ─────────────────────────────────────────────────────────────────
MODULE       := github.com/benakaben10/sns
BINARY       := notification-service
BUILD_DIR    := bin
CMD_DIR      := ./cmd/notification-service
DOCKER_IMAGE := notification-service

GO           := go
GOFLAGS      := -trimpath
LDFLAGS      := -ldflags="-w -s"

# Colours for help output
CYAN  := \033[36m
RESET := \033[0m
BOLD  := \033[1m

.DEFAULT_GOAL := help
.PHONY: help build run test test-verbose test-race lint fmt vet tidy \
        docker-build docker-push \
        up down restart logs logs-app logs-db logs-kafka \
        migrate-up migrate-seed psql \
        topics topic-send topic-result consume-send consume-result \
        jwt-token smtp-create send-test clean

# ─────────────────────────────────────────────────────────────────────────────
#  HELP
# ─────────────────────────────────────────────────────────────────────────────

## help: Show all available targets with descriptions
help:
	@echo ""
	@echo "$(BOLD)Simple Notification Service (SNS) — available targets$(RESET)"
	@echo ""
	@echo "$(BOLD)$(CYAN)Build & Run$(RESET)"
	@grep -E '^## (build|run|clean)' $(MAKEFILE_LIST) \
		| sed 's/## //' | awk -F: '{printf "  $(CYAN)make %-22s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(BOLD)$(CYAN)Testing & Code Quality$(RESET)"
	@grep -E '^## (test|lint|fmt|vet|tidy)' $(MAKEFILE_LIST) \
		| sed 's/## //' | awk -F: '{printf "  $(CYAN)make %-22s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(BOLD)$(CYAN)Docker$(RESET)"
	@grep -E '^## (docker|up|down|restart|logs)' $(MAKEFILE_LIST) \
		| sed 's/## //' | awk -F: '{printf "  $(CYAN)make %-22s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(BOLD)$(CYAN)Database$(RESET)"
	@grep -E '^## (migrate|psql)' $(MAKEFILE_LIST) \
		| sed 's/## //' | awk -F: '{printf "  $(CYAN)make %-22s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(BOLD)$(CYAN)Kafka$(RESET)"
	@grep -E '^## (topic|consume)' $(MAKEFILE_LIST) \
		| sed 's/## //' | awk -F: '{printf "  $(CYAN)make %-22s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(BOLD)$(CYAN)Utilities$(RESET)"
	@grep -E '^## (jwt-token|smtp-create|send-test)' $(MAKEFILE_LIST) \
		| sed 's/## //' | awk -F: '{printf "  $(CYAN)make %-22s$(RESET) %s\n", $$1, $$2}'
	@echo ""

# ─────────────────────────────────────────────────────────────────────────────
#  BUILD & RUN
# ─────────────────────────────────────────────────────────────────────────────

## build: Compile binary into ./bin/
build:
	@echo ">> Building $(BINARY)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)
	@echo ">> Binary: $(BUILD_DIR)/$(BINARY)"

## run: Run the service directly (requires .env file or env vars already set)
run:
	@test -f .env && export $$(grep -v '^#' .env | xargs) 2>/dev/null; \
	$(GO) run $(CMD_DIR)

## clean: Remove compiled binary and test cache
clean:
	@echo ">> Cleaning..."
	@rm -rf $(BUILD_DIR)
	$(GO) clean -testcache
	@echo ">> Done"

# ─────────────────────────────────────────────────────────────────────────────
#  TESTING & CODE QUALITY
# ─────────────────────────────────────────────────────────────────────────────

## test: Run all unit tests
test:
	$(GO) test ./... -count=1

## test-verbose: Run tests with verbose output per test case
test-verbose:
	$(GO) test ./... -v -count=1

## test-race: Run tests with race detector enabled
test-race:
	$(GO) test ./... -race -count=1

## lint: Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not found. Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	}
	golangci-lint run ./...

## fmt: Auto-format all Go source files
fmt:
	$(GO) fmt ./...

## vet: Run go vet for static analysis
vet:
	$(GO) vet ./...

## tidy: Update go.mod and go.sum, remove unused dependencies
tidy:
	$(GO) mod tidy

# ─────────────────────────────────────────────────────────────────────────────
#  DOCKER
# ─────────────────────────────────────────────────────────────────────────────

## docker-build: Build Docker image from deploy/Dockerfile (multi-stage)
docker-build:
	docker build -f deploy/Dockerfile -t $(DOCKER_IMAGE):latest .

## docker-push: Push Docker image to registry (requires REGISTRY=your.registry.io)
docker-push:
	@test -n "$(REGISTRY)" || { echo "Error: REGISTRY is not set. Usage: make docker-push REGISTRY=your.registry.io"; exit 1; }
	docker tag $(DOCKER_IMAGE):latest $(REGISTRY)/$(DOCKER_IMAGE):latest
	docker push $(REGISTRY)/$(DOCKER_IMAGE):latest

## up: Start the full stack (app + postgres + redpanda + console) with docker compose
up:
	docker compose up -d
	@echo ""
	@echo "Services started:"
	@echo "  HTTP API       → http://localhost:8080"
	@echo "  Redpanda UI    → http://localhost:8090"
	@echo "  Postgres       → localhost:5432"
	@echo ""

## down: Stop and remove containers (volumes are preserved)
down:
	docker compose down

## restart: Restart only the app container (DB and Kafka are untouched)
restart:
	docker compose restart app

## logs: Stream logs from all services
logs:
	docker compose logs -f

## logs-app: Stream logs from the app service
logs-app:
	docker compose logs -f app

## logs-db: Stream logs from postgres
logs-db:
	docker compose logs -f postgres

## logs-kafka: Stream logs from redpanda
logs-kafka:
	docker compose logs -f redpanda

# ─────────────────────────────────────────────────────────────────────────────
#  DATABASE
# ─────────────────────────────────────────────────────────────────────────────

## migrate-up: Run SQL migration to create the smtp_configs table
migrate-up:
	docker compose exec -T postgres psql -U notifsvc -d notifdb < migrations/001_create_smtp_configs.sql
	@echo ">> Migration applied"

## migrate-seed: Insert sample SMTP config seed data into the database
migrate-seed:
	docker compose exec -T postgres psql -U notifsvc -d notifdb < migrations/002_seed_smtp_config.sql
	@echo ">> Seed data inserted"

## psql: Open a psql shell connected directly to the database
psql:
	docker compose exec postgres psql -U notifsvc -d notifdb

# ─────────────────────────────────────────────────────────────────────────────
#  KAFKA / REDPANDA
# ─────────────────────────────────────────────────────────────────────────────

KAFKA_BROKER := localhost:19092

## topics: List all existing Kafka topics
topics:
	docker compose exec redpanda rpk topic list

## topic-send: Create Kafka topic email.send (partitions=3, replicas=1)
topic-send:
	docker compose exec redpanda rpk topic create email.send \
		--partitions 3 --replicas 1 || true

## topic-result: Create Kafka topic email.result (partitions=3, replicas=1)
topic-result:
	docker compose exec redpanda rpk topic create email.result \
		--partitions 3 --replicas 1 || true

## consume-send: Tail messages in real time from topic email.send
consume-send:
	docker compose exec redpanda rpk topic consume email.send --brokers localhost:9092

## consume-result: Tail messages in real time from topic email.result (send results)
consume-result:
	docker compose exec redpanda rpk topic consume email.result --brokers localhost:9092

# ─────────────────────────────────────────────────────────────────────────────
#  UTILITIES
# ─────────────────────────────────────────────────────────────────────────────

# Default values for jwt-token (override with: make jwt-token JWT_SUB=alice JWT_SCOPE="smtp:manage")
JWT_SECRET  ?= local-dev-secret-change-in-production
JWT_SUB     ?= user1
JWT_SCOPE   ?= email:send
JWT_ISSUER  ?= notification-service-local
JWT_AUD     ?= notification-service
JWT_EXP     ?= 3600

# Defined as a multi-line block to avoid try/except single-line syntax errors in shell
define JWT_PYTHON
import sys, time
try:
    import jwt
except ImportError:
    print("Install PyJWT: pip install pyjwt")
    sys.exit(1)
payload = {
    "sub":   "$(JWT_SUB)",
    "scope": "$(JWT_SCOPE)",
    "iss":   "$(JWT_ISSUER)",
    "aud":   "$(JWT_AUD)",
    "iat":   int(time.time()),
    "exp":   int(time.time()) + $(JWT_EXP),
}
token = jwt.encode(payload, "$(JWT_SECRET)", algorithm="HS256")
print(token if isinstance(token, str) else token.decode())
endef
export JWT_PYTHON

# Same as JWT_PYTHON but reads all values from environment variables (used by smtp-create / send-test)
define JWT_PYTHON_ENV
import sys, os, time
try:
    import jwt
except ImportError:
    print("pip install pyjwt")
    sys.exit(1)
payload = {
    "sub":   os.getenv("JWT_SUB",    "admin"),
    "scope": os.getenv("JWT_SCOPE",  "email:send"),
    "iss":   os.getenv("JWT_ISSUER", "notification-service-local"),
    "aud":   os.getenv("JWT_AUD",    "notification-service"),
    "iat":   int(time.time()),
    "exp":   int(time.time()) + int(os.getenv("JWT_EXP", "3600")),
}
token = jwt.encode(payload, os.getenv("JWT_SECRET", "local-dev-secret-change-in-production"), algorithm="HS256")
print(token if isinstance(token, str) else token.decode())
endef
export JWT_PYTHON_ENV

## jwt-token: Generate a JWT token for API testing (requires python3 and PyJWT: pip install pyjwt)
jwt-token:
	@command -v python3 >/dev/null 2>&1 || { echo "python3 not found. Install: sudo apt install python3"; exit 1; }
	@echo "$$JWT_PYTHON" | python3
	@echo ""
	@echo "Usage:"
	@echo "  TOKEN=\$$(make jwt-token -s)"
	@echo "  curl -X POST http://localhost:8080/api/email/raw/send \\"
	@echo "    -H \"Authorization: Bearer \$$TOKEN\" \\"
	@echo "    -H \"Content-Type: application/json\" \\"
	@echo "    -d '{\"from\":\"s@example.com\",\"to\":[\"r@example.com\"],\"title\":\"Test\",\"body\":\"Hello\"}'"

# ── API endpoint ──────────────────────────────────────────────────────────────
API_URL ?= http://localhost:8080

# smtp-create defaults (override on command line)
SMTP_NAME      ?= Gmail
SMTP_HOST      ?= smtp.gmail.com
SMTP_PORT      ?= 587
SMTP_USER      ?=
SMTP_PASS      ?=
SMTP_FROM      ?=
SMTP_TLS       ?= false
SMTP_STARTTLS  ?= true
SMTP_DEFAULT   ?= false

# send-test defaults (override on command line)
SEND_FROM  ?=
SEND_TO    ?=
SEND_TITLE ?= Test Email from SNS
SEND_BODY  ?= Hello! This is a test email from Simple Notification Service.

## smtp-create: Create an SMTP config via API (SMTP_USER and SMTP_PASS required; JWT_TOKEN auto-generated if not set)
smtp-create:
	@command -v python3 >/dev/null 2>&1 || { echo "python3 not found"; exit 1; }
	@test -n "$(SMTP_USER)" || { echo "Error: SMTP_USER required."; echo "Usage: make smtp-create SMTP_USER=you@gmail.com SMTP_PASS='app-password'"; exit 1; }
	@test -n "$(SMTP_PASS)" || { echo "Error: SMTP_PASS required."; echo "Usage: make smtp-create SMTP_USER=you@gmail.com SMTP_PASS='app-password'"; exit 1; }
	@set -e; \
	_tok=$${JWT_TOKEN:-$$(JWT_SCOPE=smtp:manage JWT_SUB="$(JWT_SUB)" JWT_SECRET="$(JWT_SECRET)" \
	         JWT_ISSUER="$(JWT_ISSUER)" JWT_AUD="$(JWT_AUD)" JWT_EXP="$(JWT_EXP)" \
	         sh -c 'echo "$$JWT_PYTHON_ENV" | python3')}; \
	_body=$$(SMTP_NAME="$(SMTP_NAME)" SMTP_HOST="$(SMTP_HOST)" SMTP_PORT="$(SMTP_PORT)" \
	         SMTP_USER="$(SMTP_USER)" SMTP_PASS="$(SMTP_PASS)" SMTP_FROM="$(SMTP_FROM)" \
	         SMTP_TLS="$(SMTP_TLS)" SMTP_STARTTLS="$(SMTP_STARTTLS)" SMTP_DEFAULT="$(SMTP_DEFAULT)" \
	         python3 -c 'import json,os; print(json.dumps({"name":os.getenv("SMTP_NAME","Gmail"),"host":os.getenv("SMTP_HOST","smtp.gmail.com"),"port":int(os.getenv("SMTP_PORT","587")),"username":os.getenv("SMTP_USER"),"password":os.getenv("SMTP_PASS"),"from_email":os.getenv("SMTP_FROM",""),"use_tls":os.getenv("SMTP_TLS","false")=="true","use_starttls":os.getenv("SMTP_STARTTLS","true")=="true","is_default":os.getenv("SMTP_DEFAULT","false")=="true"}))'); \
	echo ">> Creating SMTP config '$(SMTP_NAME)' ($(SMTP_HOST):$(SMTP_PORT))..."; \
	_resp=$$(curl -sf -X POST $(API_URL)/api/smtp-configs \
	         -H "Authorization: Bearer $$_tok" \
	         -H "Content-Type: application/json" \
	         -d "$$_body"); \
	echo "$$_resp" | python3 -m json.tool 2>/dev/null || echo "$$_resp"

## send-test: Send a test email via API (SEND_FROM, SEND_TO required; JWT_TOKEN auto-generated if not set)
send-test:
	@command -v python3 >/dev/null 2>&1 || { echo "python3 not found"; exit 1; }
	@test -n "$(SEND_FROM)" || { echo "Error: SEND_FROM required."; echo "Usage: make send-test SEND_FROM=from@gmail.com SEND_TO=to@example.com"; exit 1; }
	@test -n "$(SEND_TO)"   || { echo "Error: SEND_TO required.";   echo "Usage: make send-test SEND_FROM=from@gmail.com SEND_TO=to@example.com"; exit 1; }
	@set -e; \
	_tok=$${JWT_TOKEN:-$$(JWT_SCOPE=email:send JWT_SUB="$(JWT_SUB)" JWT_SECRET="$(JWT_SECRET)" \
	         JWT_ISSUER="$(JWT_ISSUER)" JWT_AUD="$(JWT_AUD)" JWT_EXP="$(JWT_EXP)" \
	         sh -c 'echo "$$JWT_PYTHON_ENV" | python3')}; \
	_body=$$(SEND_FROM="$(SEND_FROM)" SEND_TO="$(SEND_TO)" \
	         SEND_TITLE="$(SEND_TITLE)" SEND_BODY="$(SEND_BODY)" \
	         python3 -c 'import json,os; to=[e.strip() for e in os.getenv("SEND_TO","").split(",") if e.strip()]; print(json.dumps({"from":os.getenv("SEND_FROM"),"to":to,"title":os.getenv("SEND_TITLE","Test Email"),"body":os.getenv("SEND_BODY","Hello!")}))'); \
	echo ">> Sending: $(SEND_FROM) → $(SEND_TO)"; \
	echo ">> Title  : $(SEND_TITLE)"; \
	_resp=$$(curl -sf -X POST $(API_URL)/api/email/raw/send \
	         -H "Authorization: Bearer $$_tok" \
	         -H "Content-Type: application/json" \
	         -d "$$_body"); \
	echo "$$_resp" | python3 -m json.tool 2>/dev/null || echo "$$_resp"

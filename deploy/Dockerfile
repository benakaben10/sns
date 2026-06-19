FROM golang:1.26-alpine AS builder

WORKDIR /opt/app

RUN apk add --no-cache git tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /opt/app/notification-service \
    ./cmd/notification-service

# ── Final stage ───────────────────────────────────────────────────────────────
FROM alpine:3.19

# Copy certs + timezone from builder — avoids apk add network calls in final stage
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo                /usr/share/zoneinfo

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /opt/app
ENV WORK_DIR=/opt/app

COPY --from=builder /opt/app/notification-service .
COPY migrations/ migrations/
COPY deploy/    deploy/

RUN chmod -R a+x /opt/app/deploy/entrypoints && \
    chmod a+x /opt/app/deploy/entrypoint.sh

USER appuser

EXPOSE 8080
EXPOSE 10254

ENTRYPOINT ["/opt/app/deploy/entrypoint.sh"]

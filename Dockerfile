# syntax=docker/dockerfile:1

# Malawi E-Commerce Store — multistage image.
#
# Targets:
#   dev         live reload with Air (mount the repo at /app at runtime)
#   staging     production-shaped image, debug-friendly defaults
#   production  hardened defaults (release mode, non-root, healthcheck)
#
# The API is the default entrypoint. Run the worker by overriding it:
#   docker run ... <image> /app/worker            (with --no-healthcheck,
#   since /health only exists on the API)
#
# Migrations (db/migrations/*.sql, River tables via `river migrate-up`)
# are applied OUTSIDE the image (CI migrate step or psql) — the image
# only ships the files under /app/db/migrations for reference.

ARG GO_VERSION=1.27

# ---- base: toolchain + cached module downloads ----
FROM golang:${GO_VERSION}-alpine AS base
WORKDIR /app
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download

# ---- dev: Air live reload, source mounted at runtime ----
FROM base AS dev
RUN go install github.com/air-verse/air@latest
# Mounted source keeps host ownership; without this git (VCS stamping)
# refuses to run as a different uid and every build fails.
RUN git config --global --add safe.directory /app
COPY .air.toml ./
EXPOSE 8080
# Example: docker run --rm -v .:/app -p 8080:8080 --env-file .env <image>
CMD ["air", "-c", ".air.toml"]

# ---- builder: static binaries for api + worker ----
FROM base AS builder
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
      -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
      -o /out/worker ./cmd/worker

# ---- runtime: minimal shared base for staging + production ----
FROM alpine:3 AS runtime
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -h /app appuser
WORKDIR /app
COPY --from=builder /out/api /out/worker /app/
COPY --from=builder /app/db/migrations /app/db/migrations
USER appuser
EXPOSE 8080

# ---- staging: production shape, debug-friendly ----
FROM runtime AS staging
ENV APP_ENV=staging
ENTRYPOINT ["/app/api"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s \
  CMD wget -qO- http://localhost:8080/health || exit 1

# ---- production: hardened defaults ----
FROM runtime AS production
ENV APP_ENV=production
ENTRYPOINT ["/app/api"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s \
  CMD wget -qO- http://localhost:8080/health || exit 1

# syntax=docker/dockerfile:1
#
# One image, three binaries. The api / worker / mcp entrypoints all live in this
# module and share the embedded goose migrations + docsurface assets, so we build
# them together and let docker-compose.prod.yml pick the command per service.
#
#   command: ["api"]     # HTTP API  (:8000 in-container)
#   command: ["worker"]  # Asynq queue consumer + enrichment pipeline
#   command: ["mcp"]     # MCP page server (:8090 in-container)

# --- build stage ------------------------------------------------------------
FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
# Cache deps first: only re-downloads when go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Pure-Go (pgx, no cgo) — static binaries so the runtime can be plain alpine.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api    ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mcp    ./cmd/mcp

# --- runtime stage ----------------------------------------------------------
FROM alpine:3.20
# ca-certificates: outbound TLS (OpenAI, esm vendoring). wget: healthchecks.
RUN apk add --no-cache ca-certificates tzdata wget \
 && adduser -D -u 10001 aladin \
 && mkdir -p /data && chown aladin:aladin /data
COPY --from=build /out/api /out/worker /out/mcp /usr/local/bin/
USER aladin
ENV DATA_VOLUME_PATH=/data
# Overridden per-service in compose; api is the sensible default.
CMD ["api"]

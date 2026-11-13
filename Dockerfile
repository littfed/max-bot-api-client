# ── Stage 1: Build Showcase Bot Binary ─────────────────────────────
FROM golang:alpine AS builder

ENV GOTOOLCHAIN=auto
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Copy module manifests
COPY go.mod ./
COPY go.su[m] ./

# Copy complete repository (including certs/ with embedded Минцифры TLS certificates)
COPY . .

# Build statically linked binary without CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /bin/showcase-bot ./cmd/showcase-bot

# ── Stage 2: Minimal Distroless/Alpine Runtime ─────────────────────
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Run as non-root user
RUN adduser -D -g '' appuser

COPY --from=builder /bin/showcase-bot /app/showcase-bot

# Expose Mini App / Webhook / Health check HTTP port
EXPOSE 8080

USER appuser

ENTRYPOINT ["/app/showcase-bot"]
CMD ["-port", "8080"]

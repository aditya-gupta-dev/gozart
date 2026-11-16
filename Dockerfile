# ---- Build stage: compile a small static Go binary ----
FROM golang:1.26.1-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# SQLite driver is pure Go (modernc.org/sqlite), so CGO can be disabled
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gozart .

# ---- Runtime stage: slim image for Render free tier (512MB RAM, no disk) ----
FROM alpine:3.22

RUN apk add --no-cache ffmpeg yt-dlp ca-certificates tzdata \
    && adduser -D -u 10001 gozart

WORKDIR /app

COPY --from=builder /out/gozart /usr/local/bin/gozart
COPY --chown=gozart:gozart assets ./assets

# Free tier has no persistent disk: data resets on every deploy/restart
ENV GOZART_DB_PATH=/app/data/gozart.db \
    PORT=8080 \
    GOMEMLIMIT=400MiB \
    GOMAXPROCS=1

RUN mkdir -p /app/data /app/output /app/workspace \
    && chown -R gozart:gozart /app

USER gozart

EXPOSE 8080

# Render injects $PORT at runtime; shell form lets it expand
CMD gozart web -H 0.0.0.0 -p ${PORT}

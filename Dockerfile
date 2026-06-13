FROM golang:1.26.1-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gozart .

FROM alpine:latest

RUN apk add --no-cache \
    ffmpeg \
    python3 \
    py3-pip \
    ca-certificates

RUN pip3 install --no-cache-dir yt-dlp --break-system-packages

WORKDIR /workspace

COPY --from=builder /app/gozart /usr/local/bin/gozart

RUN chmod +x /usr/local/bin/gozart

ENTRYPOINT ["gozart"]
CMD ["--help"]

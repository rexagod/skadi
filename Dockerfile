FROM golang:1.24.4 AS builder

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /main ./cmd

# Generate self-signed TLS cert and key in a separate stage
FROM debian:stable-slim AS certgen

RUN apt-get update && apt-get install -y openssl && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /tmp/tls \
    && openssl req -x509 -newkey rsa:4096 -keyout /tmp/tls/tls.key -out /tmp/tls/tls.crt -days 365 -nodes -subj "/CN=metrics-server.kube-system.svc" \
    && chmod 755 /tmp/tls \
    && chmod 644 /tmp/tls/tls.crt /tmp/tls/tls.key

# Final image
FROM curlimages/curl

COPY --from=builder /main /main
COPY --from=certgen /tmp/tls /tmp/tls

EXPOSE 8002

ENTRYPOINT ["/main"]



# Build Go binary
FROM golang:1.24.4 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /main ./cmd

# Run binary
FROM gcr.io/distroless/base-debian12
COPY --from=builder /main /main
EXPOSE 8002
ENTRYPOINT ["/main"]

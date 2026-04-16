# Build Stage
FROM golang:1.24-alpine AS builder

# Install build dependencies for CGO (required by sqlite3)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Build the binary with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -o t-guard main.go

# Final Stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/t-guard .
COPY --from=builder /app/config.example.yaml ./config.yaml

# Create data directory
RUN mkdir -p /app/data

EXPOSE 8080
ENTRYPOINT ["./t-guard"]

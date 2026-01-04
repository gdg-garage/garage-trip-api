# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies (needed for go-sqlite3)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=1 is required for go-sqlite3
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./cmd/server/main.go

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .
# Copy .env file if available (optional, but good for defaults if not provided via docker-compose)
# COPY .env . 

# Create volume mount point for sqlite db
VOLUME ["/app/data"]

EXPOSE 8080

CMD ["./server"]

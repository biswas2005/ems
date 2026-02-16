# ----------- STAGE 1: Build -----------
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git (required for go modules)
RUN apk add --no-cache git

# Copy dependency files first (layer caching optimization)
COPY go.mod go.sum ./
RUN go mod download

# Copy project source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ems

# ----------- STAGE 2: Runtime -----------
FROM alpine:3.19

WORKDIR /app

# Create non-root user (security best practice)
RUN adduser -D appuser

# Copy compiled binary from builder
COPY --from=builder /app/ems .

# Expose application port
EXPOSE 8080

# Switch to non-root user
USER appuser

# Run the binary
CMD ["./ems"]

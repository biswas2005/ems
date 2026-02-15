FROM golang:latest AS builder


WORKDIR /app

RUN apt-get update && apt-get install -y git

# Copy go mod files first
COPY go.mod go.sum ./
RUN go mod download

# Copy entire project
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ems

FROM alpine:latest

WORKDIR /app

# Copy compiled binary
COPY --from=builder /app/ems .

# Copy .env if you want (optional)
COPY .env .

EXPOSE 8080

CMD ["./ems"]

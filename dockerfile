# Stage 1: Build your Go application
FROM golang:1.22 AS builder

# Install CA certificates
RUN apt-get update && apt-get install -y ca-certificates

# Set the working directory
WORKDIR /app

# Copy Go modules and install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o iot-gateway main.go

# Stage 2: Set up the runtime environment
FROM debian:bookworm

# Install CA certificates
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Set up working directory
WORKDIR /app

# Copy the Go binary from the builder stage
COPY --from=builder /app/iot-gateway /app/iot-gateway

# Copy other required files
COPY config.json ./
COPY server.crt server.key ./
COPY webui/templates /app/webui/templates
COPY webui/assets /app/webui/assets

# Expose necessary ports
EXPOSE 8080 8443 50000 5001 5101 5100

# Command to run the application
CMD ["/app/iot-gateway"]

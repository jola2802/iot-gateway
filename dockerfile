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

# Install CA certificates and Node.js with npm
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl && \
    curl -fsSL https://deb.nodesource.com/setup_18.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

#Install Node-RED globally
RUN npm install -g --unsafe-perm node-red

# Install additionally Node-RED nodes
RUN npm install -g \
    node-red-dashboard \
    node-red-node-sqlite \
    node-red-contrib-opcua \
    node-red-contrib-s7 \
    node-red-contrib-modbus \
    node-red-contrib-image-output

# Set up working directory
WORKDIR /app

# Copy the Go binary from the builder stage
COPY --from=builder /app/iot-gateway /app/iot-gateway

# Copy other required files
COPY config.json ./
COPY server.crt server.key ./
COPY webui/templates /app/webui/templates
COPY webui/assets /app/webui/assets
COPY iot_gateway.db /app/iot_gateway.db

# Copy Node-RED flow configuration
COPY flows.json /data/flows.json

# Expose necessary ports
EXPOSE 8443 50000 5001 5101 5100 7777 1880

# Command to run both the Go application and Node-RED
CMD ["/bin/sh", "-c", "/app/iot-gateway --tls-keyfile /data/server.key --tls-certfile /data/server.crt --listen 0.0.0.0:8443 & node-red -u /data --tls-keyfile /data/server.key --tls-certfile /data/server.crt --listen 0.0.0.0:7777"]
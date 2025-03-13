# Stage 1: Build your Go application
FROM golang:1.24 AS builder

# Install CA certificates
# RUN apt-get update && apt-get install -y ca-certificates

# Set the working directory
WORKDIR /app

# Copy Go modules and install dependencies
COPY go.mod go.sum ./ 
RUN go mod download

# Copy the source code
COPY . .
# Build the Go application
# RUN go build -o iot-gateway main.go
# Build the Go application statisch (CGO deaktiviert)
RUN CGO_ENABLED=0 go build -o iot-gateway main.go


# Stage 2: Set up the runtime environment
# FROM debian:bookworm-slim
FROM alpine:latest

# Füge tzdata hinzu, damit Zeitzoneninformationen verfügbar sind
RUN apk add --no-cache tzdata

# set timezone Europe/Berlin
RUN cp /usr/share/zoneinfo/Europe/Berlin /etc/localtime

# WORKDIR /data
# # Kopiere die Dateien korrekt in das Verzeichnis
# COPY server.crt server.key ./data/
# COPY ./data/settings.js /data/settings.js
# COPY ./data/flows.json /data/flows.json

# # Install dependencies
# RUN apt-get update && apt-get install -y \
#     ca-certificates \
#     curl \
#     wget \
#     tar \
#     gzip && \
#     curl -fsSL https://deb.nodesource.com/setup_18.x | bash - && \
#     apt-get install -y nodejs && \
#     apt-get clean && \
#     rm -rf /var/lib/apt/lists/*

# # Install Node-RED globally
# RUN npm install -g --unsafe-perm node-red

# # Install additional Node-RED nodes
# RUN npm install -g \
#     node-red-dashboard \
#     node-red-node-sqlite \
#     node-red-contrib-opcua \
#     node-red-contrib-s7 \
#     node-red-contrib-modbus \
#     node-red-contrib-image-output \
#     node-red-contrib-influxdb

# # Install InfluxDB (v2)
# RUN wget https://dl.influxdata.com/influxdb/releases/influxdb2-2.7.0-linux-amd64.tar.gz && \
#     tar xvzf influxdb2-2.7.0-linux-amd64.tar.gz && \
#     mv influxdb2_linux_amd64 /usr/local/influxdb && \
#     rm influxdb2-2.7.0-linux-amd64.tar.gz

# # Add InfluxDB binaries to PATH
# ENV PATH="/usr/local/influxdb/usr/bin:${PATH}"

# ENV DOCKER_INFLUXDB_INIT_MODE=setup \
#     DOCKER_INFLUXDB_INIT_USERNAME=admin \
#     DOCKER_INFLUXDB_INIT_PASSWORD=abc+1247 \
#     DOCKER_INFLUXDB_INIT_ORG=idpm \
#     DOCKER_INFLUXDB_INIT_BUCKET=gateway \
#     DOCKER_INFLUXDB_INIT_ADMIN_TOKEN=secret-token

# Set up working directory
WORKDIR /app

# Copy the Go binary from the builder stage
COPY --from=builder /app/iot-gateway .

# Copy other required files
# COPY server.crt server.key ./
# COPY webui/templates /app/webui/templates
# COPY webui/assets /app/webui/assets
# COPY iot_gateway.db /app/iot_gateway.db

RUN chmod +x /app/iot-gateway

# Copy Node-RED flow configuration
# COPY flows.json /data/flows.json

# Expose necessary ports
# EXPOSE 8443 5000 5001 5101 5100 
EXPOSE 8088 5000 5001 5100 5101
# 7777 8086

# Command to run Go application, Node-RED, and InfluxDB
# CMD ["/app/iot-gateway", "--tls-keyfile", "/data/server.key", "--tls-certfile", "/data/server.crt", "--listen", "0.0.0.0:8443"]
ENTRYPOINT [ "/app/iot-gateway" ]
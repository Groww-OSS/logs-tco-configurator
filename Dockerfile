FROM grafana/promtail:2.9.12 AS promtail


# Use the official Golang image to build the application
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o configurator .
# CMD ["/bin/sh", "-c", "/app/configurator"]



# Promtail requires debian or ubuntu as the base image to support systemd journal reading
FROM ubuntu:24.04

# tzdata required for the timestamp stage to work
# Install dependencies needed at runtime.
RUN  apt-get update \
&&  apt-get install -qy libsystemd-dev tzdata ca-certificates \
&&  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Set the working directory
WORKDIR /app

COPY --from=builder /app/configurator /app/configurator
COPY --from=promtail /usr/bin/promtail /app/promtail

RUN chmod +x /app/configurator
RUN chmod +x /app/promtail

CMD ["/app/configurator"]
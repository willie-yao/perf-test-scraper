# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code into the container
COPY *.go ./

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o perf-test-scraper

# Build production image
FROM gcr.io/distroless/static:nonroot

# Set working directory
WORKDIR /app

COPY --from=builder /app/perf-test-scraper /app/perf-test-scraper

# Expose port 8080 to the outside world
EXPOSE 8080

ENTRYPOINT [ "/app/perf-test-scraper" ]

# syntax=docker/dockerfile:1
FROM golang:1.23-alpine

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code into the container
COPY *.go ./

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o /perf-test-scraper

# Expose port 8080 to the outside world
EXPOSE 8080

# Command to run the executable
CMD ["/perf-test-scraper"]

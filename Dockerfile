# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
# Copy go mod and sum files
COPY go.mod go.sum ./
# Download all dependencies
RUN go mod download
# Copy the source code
COPY . .
# Build the Go app (ensure static linking for libc)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags '-extldflags "-static"' -o vault-unsealer

# Final stage: Use an Ubuntu base image
FROM ubuntu:20.04
WORKDIR /app
# Copy the binary from the builder stage
COPY --from=builder /app/vault-unsealer .

# Command to run the executable
CMD ["./vault-unsealer"]

# Build stage
FROM golang:1.27-alpine AS builder
WORKDIR /src

# Download dependencies first for better layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/vault-unsealer .

# Final stage: minimal, non-root image that still ships CA certificates so the
# unsealer can verify Vault's TLS endpoints.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/vault-unsealer /usr/local/bin/vault-unsealer
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/vault-unsealer"]

# BUILD STAGE
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/heimly ./cmd/heimly

# RUNTIME STAGE
FROM alpine:3.24

LABEL org.opencontainers.image.source="https://github.com/heimly-space/backend"
LABEL org.opencontainers.image.description="Heimly backend"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /app

# Copy binary from builder
COPY --from=builder /out/heimly .

# Expose port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./heimly"]

# BUILD STAGE
FROM golang:alpine AS builder

WORKDIR /app

# Cache dependencies first
COPY go.mod ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build static binary
RUN go build -o heimly ./cmd/heimly

# RUNTIME STAGE
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/heimly .

# Expose port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./heimly"]

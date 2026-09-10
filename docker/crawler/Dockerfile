# Stage 1: Build binary
FROM golang:1.27.0-alpine AS builder

# Upgrade all packages
RUN apk update && apk upgrade --no-cache

# Set working directory
WORKDIR /app

# Copy module definition files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build binary
# CGO_ENABLED=0 ensures the binary is static and works on alpine
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/crawler

# Stage 2: Create final image
FROM alpine:latest

# Upgrade all packages
RUN apk update && apk upgrade --no-cache

# Chromium and its runtime dependencies: the crawler drives a headless
# browser via chromedp, which needs an actual browser binary on PATH.
RUN apk add --no-cache chromium nss freetype harfbuzz ca-certificates ttf-freefont

# Create a non-root user
RUN adduser -D appuser

WORKDIR /app

# Copy binary from previous stage
COPY --from=builder /app/main .

# Ensure the binary is owned by appuser
RUN chown -R appuser:appuser /app

# Use non-root user
USER appuser

# Run application
CMD ["./main"]

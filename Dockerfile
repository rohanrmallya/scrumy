# --- Stage 1: Build Frontend ---
FROM node:24-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# --- Stage 2: Build Backend ---
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app
# Install gcc and musl-dev for CGO (required for SQLite)
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy built frontend assets for embedding
COPY --from=frontend-builder /app/web/dist ./web/dist

RUN CGO_ENABLED=1 GOOS=linux go build -o scrumy ./cmd/server/main.go

# --- Stage 3: Final Image ---
FROM alpine:latest

LABEL maintainer="Scrumy Team"
LABEL description="Scrumy Sprint Portal"

WORKDIR /app

# Install only runtime dependencies required by the binary
RUN apk add --no-cache ca-certificates libcrypto3 libssl3

# Copy the binary from the builder stage
COPY --from=backend-builder /app/scrumy .

# Expose the application port
EXPOSE 8080

# Environment variables
ENV PORT=8080

# Run as non-root user
RUN adduser -D -h /app scrumy && chown scrumy:scrumy /app
USER scrumy

# Run the application
CMD ["./scrumy"]

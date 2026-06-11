# --- Stage 1: Build Frontend ---
FROM node:24-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# --- Stage 2: Build Backend ---
FROM golang:1.26 AS backend-builder
WORKDIR /app
# Install gcc for CGO (required for SQLite)
RUN apt-get update && apt-get install -y gcc libc6-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy built frontend assets for embedding
COPY --from=frontend-builder /app/web/dist ./web/dist

RUN CGO_ENABLED=1 GOOS=linux go build -o scrumy ./cmd/server/main.go

# --- Stage 3: Final Image ---
FROM registry.access.redhat.com/ubi10/ubi-minimal:latest

LABEL maintainer="Scrumy Team"
LABEL description="Scrumy Sprint Portal"

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=backend-builder /app/scrumy .

# Expose the application port
EXPOSE 8080

# Environment variables
ENV PORT=8080
ENV GIN_MODE=release

# Run the application
CMD ["./scrumy"]

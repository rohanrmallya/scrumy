.PHONY: dev build run clean

# Run Go backend + Svelte dev server concurrently
dev:
	@echo "Starting Scrumy dev servers..."
	@trap 'kill 0' SIGINT; \
	  cd web && npm run dev & \
	  sleep 1 && go run ./cmd/server/... & \
	  wait

# Build frontend then compile Go binary
build:
	cd web && npm run build
	CGO_ENABLED=1 go build -o scrumy ./cmd/server/...

# Run built binary
run: build
	./scrumy

# Clean build artifacts
clean:
	rm -f scrumy
	rm -rf web/dist
	rm -f scrumy.db

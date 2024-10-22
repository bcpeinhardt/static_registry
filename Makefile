# We use `mkcert` to provide local development certificates
CERT_DIR := certs
CERT_KEY := $(CERT_DIR)/localhost-key.pem
CERT_CRT := $(CERT_DIR)/localhost-cert.pem

# Default target: build the project
all: build

# Generate local development certificates using mkcert
init-certificates: 
	@mkdir -p $(CERT_DIR)
	@if ! command -v mkcert > /dev/null; then echo "mkcert is not installed. Please install mkcert first."; exit 1; fi
	@mkcert -install
	@mkcert -key-file $(CERT_KEY) -cert-file $(CERT_CRT) localhost 127.0.0.1
	@echo "Certificates generated: $(CERT_CRT), $(CERT_KEY)"

# Build the Go project
build: init-certificates
	go run ./cmd/build 
	go run ./cmd/main

# Clean up certificates and build artifacts
clean:
	rm -rf $(CERT_DIR)
	rm -rf ./assets
	rm -rf ./versions.json

.PHONY: all init-certificates build clean

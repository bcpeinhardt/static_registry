BINARY_NAME := registry
MODULES_REPO_URL := https://github.com/coder/modules 
MODULES_REPO_PATH := modules

# We temporarily rename the .git folder to sneak it past 
# go:embed. If you want to rename this, you'll need to rename it
# in the Go source as well.
GIT_DIR_NAME := the_literal_git_folder

# We use `mkcert` to provide local development certificates
CERT_DIR := certs
CERT_KEY := $(CERT_DIR)/localhost-key.pem
CERT_CRT := $(CERT_DIR)/localhost-cert.pem

# Default target: build the project
all: build

# Clone the modules repository
clone-modules:
	@if [ ! -d $(MODULES_REPO_PATH) ]; then git clone $(MODULES_REPO_URL) $(MODULES_REPO_PATH); fi
	mv $(MODULES_REPO_PATH)/.git $(MODULES_REPO_PATH)/$(GIT_DIR_NAME)

# Generate local development certificates using mkcert
init-certificates: 
	@mkdir -p $(CERT_DIR)
	@if ! command -v mkcert > /dev/null; then echo "mkcert is not installed. Please install mkcert first."; exit 1; fi
	@mkcert -install
	@mkcert -key-file $(CERT_KEY) -cert-file $(CERT_CRT) localhost 127.0.0.1
	@echo "Certificates generated: $(CERT_CRT), $(CERT_KEY)"

# Build the Go project
build: clone-modules init-certificates
	go build -o $(BINARY_NAME) main.go

# Clean up certificates and build artifacts
clean: post-build-cleanup
	rm -rf $(CERT_DIR)
	rm -rf $(MODULES_REPO_PATH)
	rm -f $(BINARY_NAME)

.PHONY: all clone-modules init-certificates build post-build-cleanup clean reset

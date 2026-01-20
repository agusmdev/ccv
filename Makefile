.PHONY: build install uninstall clean test build-all

BINARY_NAME=ccv
INSTALL_DIR=/usr/local/bin
VERSION=$(shell grep 'version = ' main.go | cut -d'"' -f2)
DIST_DIR=dist

build:
	go build -o $(BINARY_NAME) .

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@if [ -w "$(INSTALL_DIR)" ]; then \
		cp $(BINARY_NAME) $(INSTALL_DIR)/; \
	else \
		sudo cp $(BINARY_NAME) $(INSTALL_DIR)/; \
	fi
	@echo "Installed successfully"

uninstall:
	@echo "Removing $(BINARY_NAME) from $(INSTALL_DIR)..."
	@if [ -w "$(INSTALL_DIR)" ]; then \
		rm -f $(INSTALL_DIR)/$(BINARY_NAME); \
	else \
		sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME); \
	fi
	@echo "Uninstalled successfully"

clean:
	rm -f $(BINARY_NAME)
	rm -rf $(DIST_DIR)

# Run Go tests
test:
	go test -v ./...

# Cross-compile for all supported platforms
build-all: clean
	@mkdir -p $(DIST_DIR)
	@echo "Building for macOS (arm64)..."
	GOOS=darwin GOARCH=arm64 go build -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "Building for macOS (amd64)..."
	GOOS=darwin GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "Building for Linux (amd64)..."
	GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .
	@echo "Building for Linux (arm64)..."
	GOOS=linux GOARCH=arm64 go build -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .
	@echo "Building for Windows (amd64)..."
	GOOS=windows GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "All builds completed in $(DIST_DIR)/"
	@ls -la $(DIST_DIR)/

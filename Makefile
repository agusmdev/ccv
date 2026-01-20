.PHONY: build install uninstall clean

BINARY_NAME=ccv
INSTALL_DIR=/usr/local/bin

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

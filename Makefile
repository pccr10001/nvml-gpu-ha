# NVML GPU Home Assistant Monitor
# Build configuration

BINARY_NAME=nvml-gpu-ha
GO_VERSION=1.21
LDFLAGS=-s -w

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .

# Build for Linux (production target)
.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux-amd64 .

# Build for Linux ARM64 (for ARM servers)
.PHONY: build-linux-arm64
build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-linux-arm64 .

# Build for Windows (development)
.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)-windows-amd64.exe .

# Download dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Run tests
.PHONY: test
test:
	go test -v ./...

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	go clean

# Install on current system
.PHONY: install
install: build
	sudo cp $(BINARY_NAME) /usr/local/bin/

# Create systemd service file
.PHONY: systemd-service
systemd-service:
	@echo "Creating systemd service file..."
	@cat > nvml-gpu-ha.service << EOF
	[Unit]
	Description=NVIDIA GPU Monitoring for Home Assistant
	After=network.target
	Wants=network.target
	
	[Service]
	Type=simple
	User=nobody
	Group=nogroup
	ExecStart=/usr/local/bin/$(BINARY_NAME)
	Restart=always
	RestartSec=10
	
	[Install]
	WantedBy=multi-user.target
	EOF
	@echo "Service file created: nvml-gpu-ha.service"
	@echo "NOTE: Configure settings in /etc/nvml-gpu-ha.conf before starting"
	@echo "To install: sudo cp nvml-gpu-ha.service /etc/systemd/system/"
	@echo "To copy config: sudo cp nvml-gpu-ha.conf.example /etc/nvml-gpu-ha.conf"
	@echo "To enable: sudo systemctl enable nvml-gpu-ha"
	@echo "To start: sudo systemctl start nvml-gpu-ha"

# Docker build
.PHONY: docker-build
docker-build:
	docker build -t nvml-gpu-ha .

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build           - Build for current platform"
	@echo "  build-linux     - Build for Linux amd64"
	@echo "  build-linux-arm64 - Build for Linux arm64"
	@echo "  build-windows   - Build for Windows amd64"
	@echo "  deps            - Download and tidy dependencies"
	@echo "  test            - Run tests"
	@echo "  clean           - Clean build artifacts"
	@echo "  install         - Install to /usr/local/bin"
	@echo "  systemd-service - Create systemd service file"
	@echo "  docker-build    - Build Docker image"
	@echo "  help            - Show this help"

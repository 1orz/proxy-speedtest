# ==============================================================================
# Project Configuration
# ==============================================================================
NAME := lite
BINDIR := bin
MODULE := github.com/1orz/proxy-speedtest

# Version info
VERSION := $(shell git describe --tags 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -X '$(MODULE)/constant.Version=$(VERSION)' \
           -X '$(MODULE)/constant.BuildTime=$(BUILD_TIME)' \
           -w -s
GOBUILD := CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)"

# Docker
DOCKER_IMAGE := $(NAME)
DOCKER_TAG := $(VERSION)

# ==============================================================================
# Platform Lists
# ==============================================================================
PLATFORM_LIST := \
	darwin-amd64 \
	darwin-amd64-v3 \
	darwin-arm64 \
	linux-386 \
	linux-amd64 \
	linux-amd64-v3 \
	linux-armv7 \
	linux-arm64 \
	freebsd-386 \
	freebsd-amd64 \
	freebsd-amd64-v3 \
	freebsd-arm64

WINDOWS_ARCH_LIST := \
	windows-386 \
	windows-amd64 \
	windows-amd64-v3 \
	windows-arm64 \
	windows-armv7

# ==============================================================================
# PHONY Targets
# ==============================================================================
.PHONY: all build run dev test lint clean help
.PHONY: docker docker-build docker-push docker-run
.PHONY: all-arch releases $(PLATFORM_LIST) $(WINDOWS_ARCH_LIST)
.PHONY: gui gui-dev

# ==============================================================================
# Default Target
# ==============================================================================
all: build

# ==============================================================================
# Development
# ==============================================================================

## build: Build for current platform
build:
	$(GOBUILD) -o $(BINDIR)/$(NAME)

## run: Build and run
run: build
	./$(BINDIR)/$(NAME)

## dev: Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	air

## test: Run tests
test:
	go test -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run linters
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Please install golangci-lint"; exit 1; }
	golangci-lint run ./...

## fmt: Format code
fmt:
	go fmt ./...
	gofmt -s -w .

## tidy: Tidy dependencies
tidy:
	go mod tidy

# ==============================================================================
# GUI
# ==============================================================================

## gui: Build GUI
gui:
	cd web/gui && npm install && npm run build

## gui-dev: Run GUI dev server
gui-dev:
	cd web/gui && npm install && npm run dev

# ==============================================================================
# Docker
# ==============================================================================

## docker: Build Docker image
docker: docker-build

## docker-build: Build Docker image
docker-build:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		.

## docker-push: Push Docker image
docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

## docker-run: Run Docker container
docker-run:
	docker run --rm -p 10888:10888 $(DOCKER_IMAGE):$(DOCKER_TAG)

# ==============================================================================
# Cross Compilation - Unix
# ==============================================================================

darwin-amd64:
	GOARCH=amd64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

darwin-amd64-v3:
	GOARCH=amd64 GOOS=darwin GOAMD64=v3 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

darwin-arm64:
	GOARCH=arm64 GOOS=darwin $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-386:
	GOARCH=386 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-amd64:
	GOARCH=amd64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-amd64-v3:
	GOARCH=amd64 GOOS=linux GOAMD64=v3 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv5:
	GOARCH=arm GOOS=linux GOARM=5 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv6:
	GOARCH=arm GOOS=linux GOARM=6 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-armv7:
	GOARCH=arm GOOS=linux GOARM=7 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-arm64:
	GOARCH=arm64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips-softfloat:
	GOARCH=mips GOMIPS=softfloat GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips-hardfloat:
	GOARCH=mips GOMIPS=hardfloat GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mipsle-softfloat:
	GOARCH=mipsle GOMIPS=softfloat GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mipsle-hardfloat:
	GOARCH=mipsle GOMIPS=hardfloat GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64:
	GOARCH=mips64 GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

linux-mips64le:
	GOARCH=mips64le GOOS=linux $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

freebsd-386:
	GOARCH=386 GOOS=freebsd $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

freebsd-amd64:
	GOARCH=amd64 GOOS=freebsd $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

freebsd-amd64-v3:
	GOARCH=amd64 GOOS=freebsd GOAMD64=v3 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

freebsd-arm64:
	GOARCH=arm64 GOOS=freebsd $(GOBUILD) -o $(BINDIR)/$(NAME)-$@

# ==============================================================================
# Cross Compilation - Windows
# ==============================================================================

windows-386:
	GOARCH=386 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-amd64:
	GOARCH=amd64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-amd64-v3:
	GOARCH=amd64 GOOS=windows GOAMD64=v3 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-arm64:
	GOARCH=arm64 GOOS=windows $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

windows-armv7:
	GOARCH=arm GOOS=windows GOARM=7 $(GOBUILD) -o $(BINDIR)/$(NAME)-$@.exe

# ==============================================================================
# Release
# ==============================================================================

## all-arch: Build for all platforms
all-arch: $(PLATFORM_LIST) $(WINDOWS_ARCH_LIST)

gz_releases := $(addsuffix .gz, $(PLATFORM_LIST))
zip_releases := $(addsuffix .zip, $(WINDOWS_ARCH_LIST))

$(gz_releases): %.gz : %
	chmod +x $(BINDIR)/$(NAME)-$(basename $@)
	gzip -f -S -$(VERSION).gz $(BINDIR)/$(NAME)-$(basename $@)

$(zip_releases): %.zip : %
	zip -m -j $(BINDIR)/$(NAME)-$(basename $@)-$(VERSION).zip $(BINDIR)/$(NAME)-$(basename $@).exe

## releases: Build and package all releases
releases: $(gz_releases) $(zip_releases)

# ==============================================================================
# Cleanup
# ==============================================================================

## clean: Remove build artifacts
clean:
	@rm -rf $(BINDIR) 2>/dev/null || true
	@rm -f coverage.out coverage.html 2>/dev/null || true
	@echo "Cleaned build artifacts"

## clean-all: Remove all generated files including GUI
clean-all: clean
	@rm -rf web/gui/dist web/gui/node_modules 2>/dev/null || true
	@echo "Cleaned all generated files"

# ==============================================================================
# Help
# ==============================================================================

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'
	@echo ""
	@echo "Cross-compilation targets:"
	@echo "  darwin-amd64, darwin-arm64, linux-amd64, linux-arm64, windows-amd64, etc."
	@echo ""
	@echo "Examples:"
	@echo "  make build        # Build for current platform"
	@echo "  make run          # Build and run"
	@echo "  make docker       # Build Docker image"
	@echo "  make all-arch     # Build for all platforms"
	@echo "  make releases     # Build and package releases"

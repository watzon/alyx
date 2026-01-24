# Alyx Makefile
# https://github.com/watzon/alyx

.PHONY: all build clean test lint fmt vet run dev install help

# Build variables
BINARY_NAME=alyx
BUILD_DIR=build
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}"

# Go variables
GOBIN?=$(shell go env GOPATH)/bin
GOFMT=gofmt
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

# Default target
all: lint test build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary
build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BUILD_DIR}
	go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} ./cmd/alyx

## build-all: Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p ${BUILD_DIR}
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-linux-amd64 ./cmd/alyx
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-linux-arm64 ./cmd/alyx
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-darwin-amd64 ./cmd/alyx
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-darwin-arm64 ./cmd/alyx
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-windows-amd64.exe ./cmd/alyx

## install: Install the binary to GOBIN
install: build
	@echo "Installing ${BINARY_NAME} to ${GOBIN}..."
	@cp ${BUILD_DIR}/${BINARY_NAME} ${GOBIN}/${BINARY_NAME}

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf ${BUILD_DIR}
	@go clean -cache

## test: Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

## test-short: Run short tests only
test-short:
	@echo "Running short tests..."
	go test -v -short ./...

## test-coverage: Run tests and show coverage
test-coverage: test
	@echo "Opening coverage report..."
	go tool cover -html=coverage.out

## lint: Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@$(GOFMT) -s -w $(GOFILES)
	@goimports -w -local github.com/watzon/alyx $(GOFILES)

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## tidy: Tidy go modules
tidy:
	@echo "Tidying go modules..."
	go mod tidy

## run: Run the development server
run: build
	@echo "Running ${BINARY_NAME}..."
	./${BUILD_DIR}/${BINARY_NAME} dev

## dev: Run with hot reload (requires air)
dev:
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not installed. Run: go install github.com/air-verse/air@latest"; \
		echo "Falling back to regular run..."; \
		$(MAKE) run; \
	fi

## generate: Run go generate
generate:
	@echo "Running go generate..."
	go generate ./...

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download

## deps-update: Update dependencies
deps-update:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

## vuln: Run vulnerability check
vuln:
	@echo "Running vulnerability check..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not installed. Run: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
		exit 1; \
	fi

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t alyx:$(VERSION) .

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 8090:8090 alyx:$(VERSION)

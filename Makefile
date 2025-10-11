BINARY=deespec
OUT=./dist/$(BINARY)
VERSION=$(shell cat VERSION 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X github.com/YoshitsuguKoike/deespec/internal/buildinfo.Version=v$(VERSION)"

.PHONY: build clean test test-coverage coverage-check coverage-html lint fmt vet version

# Show current version from VERSION file
version:
	@echo "v$(VERSION)"

build:
	@mkdir -p dist
	@echo "Building $(BINARY) v$(VERSION)..."
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(OUT) ./cmd/deespec
	@echo "✅ Built $(OUT)"

clean:
	rm -rf dist coverage.txt coverage.html

# Testing targets
test:
	go test -v -race ./...

test-coverage:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic -p 1 ./...

# Coverage validation
coverage-check: test-coverage
	@bash scripts/coverage_check.sh

coverage-html: test-coverage
	@bash scripts/coverage_check.sh --html

# Code quality targets
lint:
	go vet ./...
	gofmt -s -l .

fmt:
	gofmt -s -w .

vet:
	go vet ./...

# Development targets
dev-test: lint test-coverage coverage-check
	@echo "All development checks passed!"

ci-test: test-coverage coverage-check
	@echo "CI test suite completed!"

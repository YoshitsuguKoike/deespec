BINARY=deespec
OUT=./dist/$(BINARY)
VERSION?=dev
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

.PHONY: build clean
build:
	@mkdir -p dist
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(OUT) ./cmd/deespec

clean:
	rm -rf dist

BINARY=k10s
BUILD_DIR=./bin

.PHONY: build install clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) .

install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf $(BUILD_DIR) dist/

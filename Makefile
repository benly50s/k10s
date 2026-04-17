BINARY=k10s
BUILD_DIR=./bin
VERSION ?= 0.0.0-dev

.PHONY: build install clean completions snapshot windows-installer

build:
	go build -o $(BUILD_DIR)/$(BINARY) .

install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

snapshot:
	goreleaser release --snapshot --clean

completions:
	mkdir -p completions
	go run . completion bash > completions/k10s.bash
	go run . completion zsh  > completions/k10s.zsh
	go run . completion fish > completions/k10s.fish

windows-installer:
	@command -v makensis >/dev/null 2>&1 || { echo "makensis not installed (brew install makensis)"; exit 1; }
	bash scripts/build-windows-installer.sh $(VERSION)

clean:
	rm -rf $(BUILD_DIR) dist/

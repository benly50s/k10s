BINARY=k10s
BUILD_DIR=./bin

.PHONY: build install clean completions

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

clean:
	rm -rf $(BUILD_DIR) dist/

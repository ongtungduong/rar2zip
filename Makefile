BINARY := rar2zip
BIN_DIR := bin

.PHONY: build test vet fmt run clean

build:
	go build -o $(BIN_DIR)/$(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

# Usage: make run RAR=path/to/archive.rar
run: build
	./$(BIN_DIR)/$(BINARY) $(RAR)

clean:
	rm -rf $(BIN_DIR)

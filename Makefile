build:
	go build -o bin/pandora ./cmd/pandora

run:
	go run ./cmd/pandora

vet:
	go vet ./...

test:
	go test ./...

.PHONY: build run vet test

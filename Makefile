.PHONY: build test lint clean

build:
	go build -o mneme ./cmd/mneme

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f mneme

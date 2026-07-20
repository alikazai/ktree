.PHONY: build run test clean

build:
	go build -o ktree ./cmd/ktree

run:
	go run ./cmd/ktree

test:
	go test ./...

clean:
	rm -f ktree

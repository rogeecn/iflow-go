.PHONY: build run test clean

build:
	go build -o bin/iflow-go .

run:
	go run .

test:
	go test ./... -v

clean:
	rm -rf bin/

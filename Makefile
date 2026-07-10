.PHONY: all test fmt vet cover clean

all: test

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

cover:
	go test -coverprofile=cover.out ./...
	go tool cover -func=cover.out

clean:
	rm -f cover.out

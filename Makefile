export CGO_ENABLED := 0

.PHONY: test vet build check

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./...

check: vet test build

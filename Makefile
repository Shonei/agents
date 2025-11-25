.PHONY: build
build:
	go build -o bin/ ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix

.PHONY: test
test:
	go test -v ./...

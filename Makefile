.PHONY: build
build:
	go build -o bin/jobpin .

.PHONY: test
test:
	CGO_ENABLED=0 go test ./...

.PHONY: lint
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		go vet ./...; \
	fi

.PHONY: run
run:
	@set -a; [ -f .env ] && . ./.env; set +a; go run .

.PHONY: up
up:
	docker compose up -d

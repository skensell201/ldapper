.PHONY: test lint integration tidy build

test:
	go test -race ./...

lint:
	go vet ./...
	golangci-lint run

integration:
	docker compose -f test/integration/docker-compose.yml up -d --wait
	go test -tags=integration ./test/integration/... -v
	docker compose -f test/integration/docker-compose.yml down -v

build:
	go build ./...

tidy:
	go mod tidy

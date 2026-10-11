.PHONY: run test build up down logs

run:
	go run ./cmd/api

test:
	go test ./...

build:
	go build ./cmd/api

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f api

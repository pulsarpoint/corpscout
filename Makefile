.PHONY: up down logs rebuild migrate-up migrate-down sqlc-generate test

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

rebuild:
	docker compose up -d --build scheduler crawler

migrate-up:
	docker compose run --rm migrate up

migrate-down:
	docker compose run --rm migrate down 1

sqlc-generate:
	cd scheduler && GOWORK=off sqlc generate -f ../database/sqlc.yaml

test:
	cd scheduler && GOWORK=off go test ./...
	cd crawler && python -m pytest tests/ -v

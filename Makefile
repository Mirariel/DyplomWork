run:
	go run ./cmd/server/...

build:
	go build -o bin/server ./cmd/server/...

tidy:
	go mod tidy

test:
	go test ./...

# Docker
docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-reset:
	docker compose down -v

# Міграції
migrate-up:
	go run ./cmd/migrate/... -cmd up

migrate-down:
	go run ./cmd/migrate/... -cmd down -steps 1

migrate-version:
	go run ./cmd/migrate/... -cmd version

migrate-force:
	@read -p "Version: " v; go run ./cmd/migrate/... -cmd force -force $$v

.PHONY: run build tidy test \
        docker-up docker-down docker-logs docker-reset \
        migrate-up migrate-down migrate-version migrate-force

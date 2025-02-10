run:
	go run ./cmd/server/...

build:
	go build -o bin/server ./cmd/server/...

tidy:
	go mod tidy

# Міграції
migrate-up:
	go run ./cmd/migrate/... -cmd up

migrate-down:
	go run ./cmd/migrate/... -cmd down -steps 1

migrate-version:
	go run ./cmd/migrate/... -cmd version

migrate-force:
	@read -p "Version: " v; go run ./cmd/migrate/... -cmd force -force $$v

.PHONY: run build tidy migrate-up migrate-down migrate-version migrate-force

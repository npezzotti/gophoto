COMPOSE_FILE := docker-compose.yml
BIN_DIR := ./bin
BINARY := gophoto

.PHONY: start stop logs db-shell fmt test build clean
start: fmt
	docker compose -f $(COMPOSE_FILE) up -d --build
stop:
	docker compose -f $(COMPOSE_FILE) down
logs:
	docker compose -f $(COMPOSE_FILE) logs -f
db-shell:
	docker compose -f $(COMPOSE_FILE) exec db psql -U gophoto -d gophoto
fmt:
	go fmt ./...
test: fmt
	go test -v ./...
build:
	docker build -t gophoto:latest .

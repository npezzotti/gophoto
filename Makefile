COMPOSE_FILE := docker-compose.yml
BIN_DIR := ./bin
BINARY := gophoto

.PHONY: build run stop logs db-shell
build:
	go build -o $(BIN_DIR)/$(BINARY) cmd/*.go
run:
	docker compose -f $(COMPOSE_FILE) up -d --build
stop:
	docker compose -f $(COMPOSE_FILE) down
logs:
	docker compose -f $(COMPOSE_FILE) logs -f
db-shell:
	docker compose -f $(COMPOSE_FILE) exec db psql -U gophoto -d gophoto
fmt:
	go fmt ./...
test:
	go test -v ./...
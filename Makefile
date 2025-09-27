COMPOSE_FILE := docker-compose.yml
BIN_DIR := ./bin
BINARY := gophoto

.PHONY: start stop logs db-shell fmt test build clean
start:
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
build:
	go build -o $(BIN_DIR)/$(BINARY) cmd/*.go
clean:
	rm -rf $(BIN_DIR)/*

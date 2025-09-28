.PHONY: help build run test clean docker-up docker-down init-grid

help:
	@echo "Available commands:"
	@echo "  make build       - Build the grid-trading service"
	@echo "  make run         - Run the grid-trading service"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make docker-up   - Start PostgreSQL database"
	@echo "  make docker-down - Stop PostgreSQL database"
	@echo "  make init-grid   - Initialize grid levels (requires params)"

build:
	go build -o bin/grid-trading services/grid-trading/cmd/main.go

run:
	go run services/grid-trading/cmd/main.go

test:
	go test ./...

clean:
	rm -rf bin/

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

init-grid:
	@echo "Example: make init-grid SYMBOL=ETH MIN=2000 MAX=4000 STEP=200 AMOUNT=1000"
	go run services/grid-trading/cmd/main.go -init-grid \
		-symbol=$(SYMBOL) \
		-min-price=$(MIN) \
		-max-price=$(MAX) \
		-grid-step=$(STEP) \
		-buy-amount=$(AMOUNT)
.PHONY: help build build-all run-grid run-assurance run-all test clean docker-up docker-down init-grid

help:
	@echo "Available commands:"
	@echo "  make build           - Build the grid-trading service"
	@echo "  make build-all       - Build all services"
	@echo "  make run-grid        - Run the grid-trading service"
	@echo "  make run-assurance   - Run the order-assurance service"
	@echo "  make run-all         - Run both services in parallel"
	@echo "  make test            - Run tests"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make docker-up       - Start PostgreSQL database"
	@echo "  make docker-down     - Stop PostgreSQL database"
	@echo "  make init-grid       - Initialize grid levels (requires params)"

build:
	go build -o bin/grid-trading services/grid-trading/cmd/main.go

build-assurance:
	go build -o bin/order-assurance services/order-assurance/cmd/main.go

build-all: build build-assurance

run-grid:
	go run services/grid-trading/cmd/main.go

run-assurance:
	SERVER_PORT=9090 go run services/order-assurance/cmd/main.go

run-all:
	@echo "Starting both services..."
	@make run-assurance &
	@sleep 2
	@make run-grid

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
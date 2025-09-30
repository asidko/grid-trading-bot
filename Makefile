.PHONY: up down logs restart clean build test

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

clean:
	docker compose down -v

build:
	go build -o bin/grid-trading services/grid-trading/cmd/main.go
	go build -o bin/order-assurance services/order-assurance/cmd/main.go
	go build -o bin/price-monitor services/price-monitor/cmd/main.go

test:
	go test ./services/grid-trading/...
	go test ./services/order-assurance/...
	go test ./services/price-monitor/...
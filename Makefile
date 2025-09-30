.PHONY: init levels up down logs clean build test

init:
	@echo "Setting up grid trading bot..."
	@test -f .env || cp .env.example .env
	@echo "✓ Configuration ready (edit .env with your Binance API keys)"
	@echo "✓ Run 'make up' to start services"

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

levels:
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "  Create Grid Trading Levels"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  Configure your grid parameters (press Enter for defaults):"
	@echo ""
	@echo "  Trading pair (e.g., ETHUSDT, BTCUSDT):"
	@read -p "  Symbol [ETHUSDT]: " symbol; \
	echo ""; \
	echo "  Lower price boundary for grid:"; \
	read -p "  Min price [3500]: " min_price; \
	echo ""; \
	echo "  Upper price boundary for grid:"; \
	read -p "  Max price [4500]: " max_price; \
	echo ""; \
	echo "  Price difference between each grid level:"; \
	read -p "  Grid step [200]: " grid_step; \
	echo ""; \
	echo "  USDT amount to buy at each level:"; \
	read -p "  Buy amount USDT [1000]: " buy_amount; \
	symbol=$${symbol:-ETHUSDT}; \
	min_price=$${min_price:-3500}; \
	max_price=$${max_price:-4500}; \
	grid_step=$${grid_step:-200}; \
	buy_amount=$${buy_amount:-1000}; \
	echo ""; \
	echo "  Creating $$symbol grid: $$min_price - $$max_price (step: $$grid_step, amount: $$buy_amount USDT)..."; \
	curl -s -X POST http://localhost:8080/levels/init \
		-H "Content-Type: application/json" \
		-d "{\"symbol\":\"$$symbol\",\"min_price\":$$min_price,\"max_price\":$$max_price,\"grid_step\":$$grid_step,\"buy_amount\":$$buy_amount}" \
		&& echo "  ✓ Grid levels created successfully" \
		|| echo "  ✗ Failed to create grid levels"
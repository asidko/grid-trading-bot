.PHONY: init levels calc status up down stop logs clean build test

init:
	@echo "Setting up grid trading bot..."
	@test -f .env || cp .env.example .env
	@echo "✓ Configuration ready (edit .env with your Binance API keys)"
	@echo "✓ Run 'make up' to start services"

up:
	docker compose up -d --build

down:
	docker compose down

stop:
	docker compose stop

logs:
	docker compose logs -f

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
	symbol=$${symbol:-ETHUSDT}; \
	if ! echo "$$symbol" | grep -qE "(USDT)$$"; then \
		echo ""; \
		echo "  ⚠️  WARNING: Symbol '$$symbol' you provided does not end with USDT"; \
		echo "  ⚠️  The symbol may also not be found or supported on Binance."; \
		echo "  ⚠️  Valid examples: BTCUSDT, ETHUSDT, XRPUSDT, etc..."; \
		echo ""; \
		read -p "  Continue anyway? [y/N]: " confirm; \
		if [ "$$confirm" != "y" ] && [ "$$confirm" != "Y" ]; then \
			echo "  ✗ Grid creation cancelled"; \
			exit 1; \
		fi; \
	fi; \
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

calc:
	@read -p "Buy price [3500]: " buy; \
	read -p "Sell price [3700]: " sell; \
	read -p "Buy amount USDT [1000]: " amount; \
	buy=$${buy:-3500}; sell=$${sell:-3700}; amount=$${amount:-1000}; fee=0.1; \
	step=$$(printf "%.6f" $$(echo "$$sell - $$buy" | bc -l) | sed 's/0*$$//;s/\.$$//'); \
	buy_cost=$$(echo "$$amount * (1 + $$fee/100)" | bc -l); \
	coin=$$(echo "$$amount / $$buy" | bc -l); \
	sell_rev=$$(echo "$$coin * $$sell" | bc -l); \
	sell_net=$$(echo "$$sell_rev * (1 - $$fee/100)" | bc -l); \
	profit=$$(echo "$$sell_net - $$buy_cost" | bc -l); \
	pct=$$(echo "$$profit / $$buy_cost * 100" | bc -l); \
	printf "\nStep: %s | Profit: %.2f USDT (%.2f%%)\n\n" $$step $$profit $$pct

status:
	@curl -s -f -o /dev/null http://localhost:8080/health || { echo "\n❌ grid-trading service is not running (port 8080)"; echo "⚠️  Please run 'make up' to start services\n"; exit 1; }
	@curl -s -f -o /dev/null http://localhost:9090/health || { echo "\n❌ order-assurance service is not running (port 9090)"; echo "⚠️  Please run 'make up' to start services\n"; exit 1; }
	@curl -s -f -o /dev/null http://localhost:7070/health || { echo "\n❌ price-monitor service is not running (port 7070)"; echo "⚠️  Please run 'make up' to start services\n"; exit 1; }
	@echo "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "Grid Trading Status"
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@data=$$(curl -s http://localhost:8080/status); \
	[ -z "$$data" ] && echo "✗ Service unavailable" && exit 1; \
	echo "$$data" | jq -e . >/dev/null 2>&1 || { echo "✗ Service error: $$data"; exit 1; }; \
	echo "\n📊 Today: $$(echo $$data | jq -r '.buys_today') buys, $$(echo $$data | jq -r '.sells_today') sells, $$(echo $$data | jq -r '.errors_today') errors"; \
	echo "💰 Profit: $$(echo $$data | jq -r '.profit_today') today | $$(echo $$data | jq -r '.profit_this_week') week | $$(echo $$data | jq -r '.profit_this_month') month | $$(echo $$data | jq -r '.profit_all_time') total (USDT)"; \
	echo "📈 Levels: $$(echo $$data | jq -r '.waiting_for_buy') waiting for buy, $$(echo $$data | jq -r '.waiting_for_sell') waiting for sell"; \
	echo $$data | jq -e '.last_buy' > /dev/null 2>&1 && [ "$$(echo $$data | jq -r '.last_buy')" != "null" ] && { \
		echo "\n🟢 Last Buy: $$(echo $$data | jq -r '.last_buy.symbol') @ $$(echo $$data | jq -r '.last_buy.price')"; \
		echo "   Amount: $$(echo $$data | jq -r '.last_buy.amount') | Time: $$(echo $$data | jq -r '.last_buy.time')"; \
	} || true; \
	echo $$data | jq -e '.last_sell' > /dev/null 2>&1 && [ "$$(echo $$data | jq -r '.last_sell')" != "null" ] && { \
		echo "\n🔴 Last Sell: $$(echo $$data | jq -r '.last_sell.symbol') @ $$(echo $$data | jq -r '.last_sell.price')"; \
		echo "   Profit: $$(echo $$data | jq -r '.last_sell.profit_usdt') USDT ($$(echo $$data | jq -r '.last_sell.profit_pct')%) | Time: $$(echo $$data | jq -r '.last_sell.time')"; \
	} || true; \
	echo $$data | jq -e '.last_price_update' > /dev/null 2>&1 && [ "$$(echo $$data | jq -r '.last_price_update')" != "null" ] && { \
		echo "\n📍 Price: $$(echo $$data | jq -r '.last_price_update.symbol') @ $$(echo $$data | jq -r '.last_price_update.price') | $$(echo $$data | jq -r '.last_price_update.updated_at')"; \
	} || true
	@if [ -f .grid-trading-data/grid_trading.db ]; then \
		error_count=$$(sqlite3 .grid-trading-data/grid_trading.db "SELECT COUNT(*) FROM grid_levels WHERE state='ERROR'" 2>/dev/null || echo "0"); \
		if [ "$$error_count" -gt 0 ]; then \
			echo "\n⚠️  Error Levels ($$error_count):"; \
			sqlite3 -header -column .grid-trading-data/grid_trading.db \
				"SELECT g.id, g.symbol, g.buy_price, g.sell_price, t.error_code, SUBSTR(t.error_msg, 1, 40) as error_msg FROM grid_levels g LEFT JOIN (SELECT grid_level_id, error_code, error_msg FROM transactions WHERE status='ERROR' ORDER BY created_at DESC) t ON g.id = t.grid_level_id WHERE g.state='ERROR' GROUP BY g.id ORDER BY g.state_changed_at DESC" 2>/dev/null | \
				sed 's/^/   /'; \
		fi; \
	fi
	@echo "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
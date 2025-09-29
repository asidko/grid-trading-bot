# Grid Trading Bot - Core Knowledge

## Architecture
3 Go microservices: **grid-trading** (8080), **order-assurance** (9090), **price-monitor** (7070) + PostgreSQL

## State Machine
```
READY → PLACING_BUY → BUY_ACTIVE → HOLDING → PLACING_SELL → SELL_ACTIVE → READY
```

## Critical Logic
- **Buy trigger:** `price > buy_price` AND `state = READY`
- **Sell trigger:** `price < sell_price` AND `state = HOLDING` (⚠️ BUG: code uses `>=`)
- Each grid level is independent buy-sell cycle with its own state

## Database
Table: `grid_levels` - Unique on `(symbol, buy_price, sell_price)`, Check: `sell_price > buy_price`

## Key Files
- `services/grid-trading/internal/service/grid_service.go` - Core trading logic
- `services/grid-trading/internal/models/grid_level.go` - State machine & triggers
- `services/order-assurance/internal/exchange/binance_client.go` - Binance integration

## Flow
1. price-monitor polls Binance → sends trigger to grid-trading
2. grid-trading checks levels → calls order-assurance → updates state
3. order-assurance places order on Binance → sends fill notification back
4. grid-trading processes fill → updates state (HOLDING or READY)

## Key Principles
- Reactive: Orders placed only on price triggers (not proactive)
- Idempotent: Order placement cached with 0.01% tolerance
- No cache: Always read state from DB
- Recovery: Hourly sync job for stuck states
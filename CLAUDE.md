# Grid Trading Bot - Core Knowledge

## Architecture
3 Go microservices: **grid-trading** (8080), **order-assurance** (9090), **price-monitor** (7070) + SQLite

## State Machine
```
READY → PLACING_BUY → BUY_ACTIVE → HOLDING → PLACING_SELL → SELL_ACTIVE → READY
```

## Critical Logic
- **Buy trigger:** `price > buy_price` AND `state = READY`
- **Sell trigger:** `price < sell_price` AND `state = HOLDING` (⚠️ BUG: code uses `>=`)
- Each grid level is independent buy-sell cycle with its own state

## Database Tables
- **grid_levels**: State machine (mutable), Unique on `(symbol, buy_price, sell_price)`
- **transactions**: Audit log (immutable), Records all fills and errors

## Key Files
- `services/grid-trading/internal/service/grid_service.go` - Core trading logic
- `services/grid-trading/internal/models/grid_level.go` - State machine & triggers
- `services/grid-trading/internal/repository/transaction_repository.go` - Transaction recording
- `services/order-assurance/internal/exchange/binance_client.go` - Binance integration

## Data Flow
1. price-monitor polls Binance → sends trigger to grid-trading
2. grid-trading checks levels → calls order-assurance → updates state
3. order-assurance places order on Binance → sends fill notification back
4. grid-trading processes fill → updates state + records transaction

## Design Principles
- **Reactive**: Orders placed only on price triggers (not proactive)
- **Idempotent**: Order placement cached with 0.01% tolerance
- **No cache**: Always read state from DB
- **Audit trail**: All trades/errors in transactions table
- **Simple types**: Use int/decimal with zero values, not sql.Null*
- **Division safety**: Always check > 0 before division
- **Actual costs**: Use transaction history for profit calc, not recalculated values

## Development Rules
- **Schema changes**: Update migration files directly, no graceful altering (data wiped before testing)
- **Error tracking**: Record errors in transactions table, not grid_levels state
- **Transaction recording**: INSERT only, never UPDATE - immutable audit log
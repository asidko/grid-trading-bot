# Grid Trading Bot - Technical Specification

## Overview

Grid trading bot that profits from market volatility by placing buy/sell orders at fixed price intervals. The bot harvests profits from price fluctuations regardless of overall market direction.

**Core Strategy:**
- Split price range into fixed levels (e.g., every $200)
- Buy when price drops to a level
- Sell when price rises to the next level above
- Each level operates independently

**System Boundaries:**
- ✅ Bot manages: grid states, order placement calls, fill processing, database consistency
- ❌ External handles: price feeds, exchange execution, connectivity, notification retries, fee calculations, profitability analysis
- ❌ Out of scope: Exchange fee economics, minimum trade sizes, capital management, P&L tracking

## Database Schema

Each grid level represents a complete buy-sell cycle with both buy and sell prices in a single record.

**Database Constraints:**
- UNIQUE constraint on `(symbol, buy_price, sell_price)` to prevent duplicate levels
- CHECK constraint: `sell_price > buy_price`

| Column | Type | Description |
|--------|------|-------------|
| `id` | integer | Primary key, auto-increment |
| `symbol` | string | Trading symbol (e.g., 'ETHUSDT', 'BTCUSDT') |
| `buy_price` | decimal(16,8) | Price to place buy order (e.g., 3600.00000000) |
| `sell_price` | decimal(16,8) | Price to place sell order (e.g., 3800.00000000) |
| `buy_amount` | decimal(16,8) | USDT amount to buy with (e.g., 1000.00000000) |
| `filled_amount` | decimal(16,8) | Actual amount bought in coins (e.g., 0.27800000 ETH) |
| `state` | enum | Current state: READY, PLACING_BUY, BUY_ACTIVE, HOLDING, PLACING_SELL, SELL_ACTIVE, ERROR |
| `buy_order_id` | string | Exchange order ID for buy order |
| `sell_order_id` | string | Exchange order ID for sell order |
| `enabled` | boolean | Enable/disable this level (default: true) |
| `error_msg` | string | Error details when in ERROR state |
| `state_changed_at` | timestamp | When state was last changed (for timeout detection) |
| `created_at` | timestamp | When level was created |
| `updated_at` | timestamp | Last update time |

**State Machine:**
```
READY → PLACING_BUY → BUY_ACTIVE → HOLDING → PLACING_SELL → SELL_ACTIVE → READY
  ↓          ↓            ↓           ↓            ↓              ↓
 ERROR ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ←
```

## Trading Logic

### Price Trigger Processing (Reactive Strategy)

**Important:** The bot operates reactively - it only places orders when triggered by price movements. It does NOT place all orders across all levels proactively. This minimizes capital lock-up and ensures orders are placed only when price is near the level.

When price update received for a symbol:
```python
for level in get_levels(symbol):
    if price > level.buy_price and level.state == 'READY' and level.enabled:
        # Trigger buy order at level.buy_price
    elif price < level.sell_price and level.state == 'HOLDING' and level.enabled:
        # Trigger sell order at level.sell_price for level.filled_amount
```

Example: ETH price = $3700
- Checks all levels for the symbol
- May place buy at 3600 (if state=READY)
- May place sell at 3800 (if state=HOLDING with coins)
- May trigger multiple independent levels simultaneously

### Order Placement Flow

**Buy Order:**
- Condition: `state = READY` AND `enabled = true` AND `price > buy_price`
- Process:
  1. Set `state = PLACING_BUY`, update `state_changed_at = NOW()`
  2. Call order assurance service: `{symbol, price: buy_price, side: "buy", amount: buy_amount}`
  3. Success → Save `buy_order_id`, set `state = BUY_ACTIVE`, update `state_changed_at`
  4. Failure → Revert to `READY`, store error in `error_msg`, update `state_changed_at`
  5. If crash occurs: On recovery, retry assurance call (idempotent) with current DB values

**Sell Order:**
- Condition: `state = HOLDING` AND `enabled = true` AND `price < sell_price`
- Process:
  1. Set `state = PLACING_SELL`, update `state_changed_at = NOW()`
  2. Call order assurance service: `{symbol, price: sell_price, side: "sell", amount: filled_amount}`
  3. Success → Save `sell_order_id`, set `state = SELL_ACTIVE`, update `state_changed_at`
  4. Failure → Revert to `HOLDING`, store error in `error_msg`, update `state_changed_at`
  5. If crash occurs: On recovery, retry assurance call (idempotent) with current DB values

### Fill Processing

**Important:** The system assumes all fill notifications represent completely filled orders. Partial fills are not expected or handled.

**Buy Fill Notification:**
```
POST /order-fill-notification
{order_id, symbol, price, side: "buy", status: "filled", filled_amount}
```
Process:
1. Find level by `buy_order_id = order_id`
2. Check if already processed: if `state != BUY_ACTIVE`, skip (idempotent)
3. Update `state = HOLDING`
4. Store `filled_amount` (actual coins bought)
5. Clear `buy_order_id`

**Sell Fill Notification:**
```
POST /order-fill-notification
{order_id, symbol, price, side: "sell", status: "filled", filled_amount}
```
Process:
1. Find level by `sell_order_id = order_id`
2. Check if already processed: if `state != SELL_ACTIVE`, skip (idempotent)
3. Update `state = READY`
4. Clear `filled_amount` and `sell_order_id`
5. Ready for next buy-sell cycle

## API Specifications

### Order Assurance Service (External)

Base URL: `ORDER_ASSURANCE_URL` environment variable

**Place Order (Idempotent):**
```
POST /order-assurance
// Always places LIMIT orders at specified price
// IMPORTANT: Idempotent based on (symbol, price, side, amount) with 0.01% tolerance
// Buy request:  {symbol: "ETHUSDT", price: 3600, side: "buy", amount: 1000}  // amount in USDT
// Sell request: {symbol: "ETHUSDT", price: 3800, side: "sell", amount: 0.294} // amount in ETH
Response: {order_id: "exchange_123", status: "assured"} // assured = limit order placed on exchange
// Idempotency: Returns same order_id if amount within 0.01% of existing order
// Example: 1000.00 and 1000.09 USDT considered same (0.009% difference)
```

**Check Status:**
```
GET /order-status/{order_id}
Response: {order_id, status: "open|filled|cancelled", filled_amount, fill_price}
```

**Status Actions:**
- `filled`: Update state to HOLDING (buy) or READY (sell)
- `cancelled` or not found: Reset state to READY
- `open`: No action needed

### Bot Endpoints (Incoming)

**Price Trigger:**
```
POST /trigger-for-price
Body: {symbol: "ETHUSDT", price: 3753}
```

**Fill Notification:**
```
POST /order-fill-notification
Body: {order_id, symbol, price, side, status: "filled", filled_amount, fill_price}
```

**Error Notification:**
```
POST /order-fill-error-notification
Body: {order_id, symbol, side, error: "insufficient_funds"}
```
Finds level by order_id, sets state to ERROR and stores error message in `error_msg` column.

### System Methods

**Initialize Grid:**
```
init-grid-levels(symbol, min_price, max_price, grid_step, buy_amount)
// Example: init-grid-levels('ETHUSDT', 2000, 4000, 200, 1000)
// Creates grid levels with buy_price and sell_price pairs:
// Level 1: buy_price=2000, sell_price=2200
// Level 2: buy_price=2200, sell_price=2400
// Level 3: buy_price=2400, sell_price=2600
// ...
// Level N: buy_price=3800, sell_price=4000
// IMPORTANT: All levels start with state=READY
// Skips creating levels that already exist (based on buy_price, sell_price combo)
// Orders are placed only when price triggers arrive
```

**Sync Orders (Recovery & Backup Mechanism):**
```
sync-all-orders()  // Runs hourly via scheduler
// Primary purpose: Recovery mechanism for missed notifications & crashes
// - Checks all order_ids via /order-status
// - Processes any fills that occurred while bot was down
// - Timeout detection using state_changed_at field:
//   - PLACING_* > 5 minutes: Retry assurance or revert to previous state
//   - *_ACTIVE > 30 days: Check if auto-cancelled by exchange
// - For PLACING_* states without order_id: Retry assurance call (idempotent)
// - For *_ACTIVE states: Check if filled/cancelled and update accordingly
// Note: This is a backup mechanism. Normal operation relies on immediate
// fill notifications via /order-fill-notification endpoint
```

## Operational Behavior

### Concurrency & Safety
- Database row locks prevent race conditions
- PLACING_* states prevent duplicate orders (checked before placing)
- All state changes within transactions (except external API calls)
- Duplicate price triggers safe: each level operates independently
- Each level follows sequential state machine: READY → BUY → HOLD → SELL → READY
- No in-memory cache: always read current state from database
- Idempotent fill notifications: checked via current state before processing
- Grid modifications allowed anytime: affect only future orders, not active ones

### Error Recovery
- **Assurance failures:** Revert to READY state
- **Lock timeout:** Stale PLACING_* states (>1 hour old) cleared by scheduled job
- **Database failures after order placed:** Log error, manual resolution
- **Unknown order_id in notification:** Log warning, attempt match by symbol/price/side
- **ERROR state levels:** Skip trading, store reason in `error_msg`, require manual reset

### System Requirements
- SQLite database (no caching, always read from DB)
- Minimum 2 levels for operation
- Configuration via direct database manipulation
- No runtime API for level management
- Scheduled job: Runs hourly to check order statuses and clear stale locks

### Key Assumptions
- Only liquid symbols (BTC/ETH/DOGE) - complete fills guaranteed
- Orders remain open indefinitely
- External system handles notification retries
- Manual intervention for ERROR states
- Bot is purely reactive - only responds to incoming price triggers
- Fill price accepted as-is (no slippage validation)
- Fill notifications are idempotent (same order_id can be processed multiple times safely)
- Dust amounts: Not expected (selling exactly what was bought)
- Grid overlaps allowed: One level selling at price X while another buys at X is valid
- **Profit accumulation:** Profits naturally accumulate in the exchange account balance
- **No P&L tracking:** The bot does not track profits, losses, or performance metrics
- **Capital management:** Not handled by this service - assumes sufficient balance maintained externally

## Example Scenarios

### Standard Buy-Sell Cycle
```
1. Grid level: buy_price=3400, sell_price=3600, buy_amount=1000
2. Price drops to 3500: Triggers buy order at 3400
3. Buy fills: 0.294 ETH bought
   - state → HOLDING
   - filled_amount = 0.294 ETH
4. Price rises to 3700: Triggers sell order at 3600 for 0.294 ETH
5. Sell fills: 0.294 ETH sold
   - state → READY
   - filled_amount cleared
   - Ready for next cycle
```

### Multiple Levels Operating
```
Level A: buy_price=3200, sell_price=3400, state=HOLDING (0.312 ETH)
Level B: buy_price=3400, sell_price=3600, state=READY
Level C: buy_price=3600, sell_price=3800, state=HOLDING (0.277 ETH)

Price = 3500:
- Level A: Can trigger sell at 3400 (has coins)
- Level B: Can trigger buy at 3400 (is ready)
- Level C: No action (price not < 3800)

All levels operate independently and simultaneously
```

### Recovery Flow
```
1. Level in PLACING_BUY state (no order_id yet)
2. System crashes or assurance call fails
3. On recovery (manual or scheduled):
   - Retry assurance call with same params (idempotent)
   - Get order_id and continue normally
4. If assurance permanently fails: revert to READY
```

### Crash Recovery (Scheduled)
```
1. Sync job runs hourly
2. For each level with pending states:
   - PLACING_* without order_id: Retry assurance (idempotent)
   - BUY_ACTIVE/SELL_ACTIVE: Check order status
   - Process based on actual exchange state
3. Ensures database matches exchange reality
```
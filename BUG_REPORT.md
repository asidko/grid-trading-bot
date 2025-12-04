# Grid Trading Bot - Bug Report

**Date:** 2025-12-04
**Analysis by:** Claude Code

## Summary

Found **6 bugs** across the grid trading bot codebase, ranging from **CRITICAL** (trading logic errors) to **MEDIUM** (missing audit trails).

---

## CRITICAL Bugs

### BUG #1: Missing Sell Price Check in `CanPlaceSell`

**File:** `services/grid-trading/internal/models/grid_level.go:45-50`

**Problem:** The `CanPlaceSell` method receives a `currentPrice` parameter but **never uses it**. Sell orders are triggered when the level is in `HOLDING` state regardless of the current market price.

**Current Code:**
```go
func (g *GridLevel) CanPlaceSell(currentPrice decimal.Decimal) bool {
    return g.State == StateHolding &&
        g.Enabled &&
        g.FilledAmount.Valid &&
        g.FilledAmount.Decimal.GreaterThan(decimal.Zero)
    // BUG: currentPrice is never checked!
}
```

**Expected Behavior (per CLAUDE.md):** Sell trigger should be `price < sell_price AND state = HOLDING`

**Expected Code:**
```go
func (g *GridLevel) CanPlaceSell(currentPrice decimal.Decimal) bool {
    return g.State == StateHolding &&
        g.Enabled &&
        g.FilledAmount.Valid &&
        g.FilledAmount.Decimal.GreaterThan(decimal.Zero) &&
        currentPrice.LessThan(g.SellPrice)  // MISSING!
}
```

**Impact:**
- Sell orders trigger at **ANY** price when in HOLDING state
- Bot could sell at a loss during uptrends
- Completely breaks the grid trading strategy
- **Financial loss risk: HIGH**

---

## HIGH Severity Bugs

### BUG #2: `GetLevelCounts` Returns Inverted Semantics

**File:** `services/grid-trading/internal/repository/grid_level_repository.go:519-530`

**Problem:** The function returns counts for the wrong states:
- `holding` variable counts `SELL_ACTIVE` (should count `HOLDING`)
- `ready` variable counts `BUY_ACTIVE` (should count `READY`)

**Current Code:**
```go
func (r *GridLevelRepository) GetLevelCounts() (holding, ready int, err error) {
    query := `
        SELECT
            COUNT(CASE WHEN state = 'SELL_ACTIVE' THEN 1 END) as holding,  -- WRONG
            COUNT(CASE WHEN state = 'BUY_ACTIVE' THEN 1 END) as ready      -- WRONG
        FROM grid_levels
        WHERE enabled = 1
    `
```

**Expected Code:**
```go
    query := `
        SELECT
            COUNT(CASE WHEN state = 'HOLDING' THEN 1 END) as holding,
            COUNT(CASE WHEN state = 'READY' THEN 1 END) as ready
        FROM grid_levels
        WHERE enabled = 1
    `
```

**Impact:**
- Status endpoint (`/status`) reports incorrect metrics
- `WaitingForBuy` and `WaitingForSell` values are wrong
- Users cannot accurately monitor bot status

---

### BUG #3: Symbol Transformation Inconsistency (`stripUSDT`)

**File:** `services/order-assurance/internal/service/order_service.go:105,121-127`

**Problem:** Fill notifications transform `ETHUSDT` to `ETH` before sending to grid-trading, but the database stores the full symbol `ETHUSDT`.

**Current Code:**
```go
func (s *OrderService) sendFillNotification(order *models.BinanceOrder, ...) {
    notification := models.FillNotification{
        Symbol: s.stripUSDT(order.Symbol),  // Transforms ETHUSDT -> ETH
        // ...
    }
}

func (s *OrderService) stripUSDT(symbol string) string {
    if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
        return symbol[:len(symbol)-4]  // Removes "USDT" suffix
    }
    return symbol
}
```

**Impact:**
- Fill notifications have symbol mismatch with database records
- Logging/debugging shows inconsistent symbols
- Potential for future bugs if symbol is used for lookups
- Violates CLAUDE.md principle: "Pass actual domain values explicitly through the entire call chain"

**Note:** Currently lookups use `order_id` not `symbol`, so this doesn't break functionality but creates technical debt.

---

## MEDIUM Severity Bugs

### BUG #4: Division by Zero Risk in `PlaceOrder`

**File:** `services/order-assurance/internal/service/order_service.go:30-34`

**Problem:** No guard against division by zero when `req.Price` is zero.

**Current Code:**
```go
func (s *OrderService) PlaceOrder(req models.OrderRequest) (*models.OrderResponse, error) {
    quantity := req.Amount
    if req.Side == models.SideBuy {
        quantity = req.Amount.Div(req.Price)  // Panic if req.Price is zero
    }
```

**Mitigating Factor:** HTTP handler at `handlers.go:42` validates `req.Price.IsZero()`, but internal calls bypass this.

**Recommendation:** Add defensive check in `PlaceOrder`:
```go
if req.Price.IsZero() {
    return nil, fmt.Errorf("price cannot be zero")
}
```

---

### BUG #5: Missing Audit Trail on Sync Job Recovery Failures

**File:** `services/grid-trading/internal/service/grid_service.go`

**Problem:** Multiple code paths in `SyncOrders()` reset state without recording audit trail transactions. Per CLAUDE.md: "Any code path that modifies state MUST record the audit event atomically."

**Affected Lines:**

1. **Lines 453-454:** Buy order recovery failure resets to `READY` without audit
   ```go
   s.repo.UpdateState(level.ID, models.StateReady)
   log.Printf("ERROR: Failed to recover buy order...")
   // No transaction recorded!
   ```

2. **Lines 472-473:** Sell order recovery failure resets to `HOLDING` without audit
   ```go
   s.repo.UpdateState(level.ID, models.StateHolding)
   log.Printf("ERROR: Failed to recover sell order...")
   // No transaction recorded!
   ```

3. **Lines 476-477:** Missing filled amount resets to `HOLDING` without audit
   ```go
   s.repo.UpdateState(level.ID, models.StateHolding)
   // No transaction recorded!
   ```

4. **Lines 516-517:** Order not found resets state without audit
   ```go
   s.repo.UpdateState(level.ID, targetState)
   // No transaction recorded!
   ```

5. **Line 540:** Cancelled order resets state without audit
   ```go
   s.repo.UpdateState(level.ID, targetState)
   // No transaction recorded!
   ```

**Impact:**
- State changes appear in DB without history in `transactions` table
- Creates "ghost operations" - state changes with no audit trail
- Profit calculations may be inaccurate if transitions are untracked
- Debugging production issues becomes impossible

---

### BUG #6: Buy Trigger Uses `>=` Instead of `>`

**File:** `services/grid-trading/internal/models/grid_level.go:38-43`

**Problem:** CLAUDE.md specifies buy trigger as `price > buy_price`, but code uses `>=`:

**Current Code:**
```go
func (g *GridLevel) CanPlaceBuy(currentPrice decimal.Decimal) bool {
    return g.State == StateReady &&
        g.Enabled &&
        currentPrice.GreaterThanOrEqual(g.BuyPrice) &&  // Uses >=
        currentPrice.LessThan(g.SellPrice)
}
```

**Expected per CLAUDE.md:**
```go
currentPrice.GreaterThan(g.BuyPrice) &&  // Should use >
```

**Impact:** Minor - may trigger buy orders slightly earlier than intended when price exactly equals buy_price. This might be intentional behavior, but contradicts documentation.

---

## Bug Summary Table

| # | Severity | Bug | File | Lines | Impact |
|---|----------|-----|------|-------|--------|
| 1 | CRITICAL | Missing sell price check | grid_level.go | 45-50 | Sells at wrong prices |
| 2 | HIGH | GetLevelCounts inverted | grid_level_repository.go | 519-530 | Wrong status metrics |
| 3 | HIGH | Symbol transformation | order_service.go | 105, 121-127 | Data inconsistency |
| 4 | MEDIUM | Division by zero risk | order_service.go | 30-34 | Potential crash |
| 5 | MEDIUM | Missing audit trail | grid_service.go | Multiple | Lost history |
| 6 | MEDIUM | Wrong comparison operator | grid_level.go | 41 | Early trigger |

---

## Recommendations

1. **Immediate:** Fix BUG #1 (missing sell price check) - this is a **critical trading logic error**
2. **High Priority:** Fix BUG #2 (GetLevelCounts) - users see incorrect metrics
3. **High Priority:** Fix BUG #3 (stripUSDT) - remove symbol transformation
4. **Medium Priority:** Add audit trail recording in sync job failure paths
5. **Low Priority:** Align comparison operators with documentation

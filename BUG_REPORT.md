# Grid Trading Bot - Bug Report

**Date:** 2025-12-04
**Analysis by:** Claude Code
**Status:** ALL ISSUES RESOLVED (2025-12-05)

## Summary

Found **6 issues** across the grid trading bot codebase. **All have been fixed.**

---

## RESOLVED Issues

### BUG #1: Unused Price Parameter in `CanPlaceSell` [RESOLVED - BY DESIGN]

**File:** `services/grid-trading/internal/models/grid_level.go:45-50`

**Original Issue:** The `CanPlaceSell` method receives a `currentPrice` parameter but never uses it.

**Resolution:** This is correct behavior. Sell orders are placed immediately after buy fills using LIMIT orders at `sell_price`. The LIMIT order handles price targeting - no need to check current price. SPEC.md updated to reflect this design decision.

---

### BUG #2: `GetLevelCounts` Returns Inverted Semantics [FIXED]

**File:** `services/grid-trading/internal/repository/grid_level_repository.go:519-530`

**Original Issue:** Function counted wrong states (`SELL_ACTIVE`/`BUY_ACTIVE` instead of `BOUGHT`/`READY`).

**Fix Applied:**
```go
func (r *GridLevelRepository) GetLevelCounts() (waitingForSell, waitingForBuy int, err error) {
    query := `
        SELECT
            COUNT(CASE WHEN state = 'BOUGHT' THEN 1 END) as waiting_for_sell,
            COUNT(CASE WHEN state = 'READY' THEN 1 END) as waiting_for_buy
        FROM grid_levels
        WHERE enabled = 1
    `
```

**Commit:** `7a1963e fix: correct GetLevelCounts to count READY and BOUGHT states`

---

### BUG #3: Symbol Transformation Inconsistency (`stripUSDT`) [FIXED]

**File:** `services/order-assurance/internal/service/order_service.go`

**Original Issue:** `stripUSDT` function transformed `ETHUSDT` to `ETH`, causing symbol mismatch with database.

**Fix Applied:** Removed `stripUSDT` function entirely. Now uses full symbol (e.g., `ETHUSDT`) consistently throughout the system.

**Commit:** `6db079d fix: address all identified bugs from code analysis`

---

### BUG #4: Division by Zero Risk in `PlaceOrder` [FIXED]

**File:** `services/order-assurance/internal/service/order_service.go:29-30`

**Original Issue:** No guard against division by zero when `req.Price` is zero.

**Fix Applied:**
```go
func (s *OrderService) PlaceOrder(req models.OrderRequest) (*models.OrderResponse, error) {
    if req.Price.IsZero() {
        return nil, fmt.Errorf("price cannot be zero")
    }
    // ...
}
```

**Commit:** `6db079d fix: address all identified bugs from code analysis`

---

### BUG #5: Missing Audit Trail on Sync Job Recovery Failures [FIXED]

**File:** `services/grid-trading/internal/service/grid_service.go`

**Original Issue:** Multiple code paths in `SyncOrders()` reset state without recording audit trail transactions.

**Fix Applied:** Added `RecordBuyError` and `RecordSellError` calls for:
- Recovery failures (lines 455, 475, 480)
- Order not found (lines 523, 525)
- Order cancelled (lines 552, 554)

**Commit:** `6db079d fix: address all identified bugs from code analysis`

---

### BUG #6: Buy Trigger Uses `>=` Instead of `>` [RESOLVED - CODE IS CORRECT]

**File:** `services/grid-trading/internal/models/grid_level.go:41`

**Original Issue:** Documentation said `>` but code used `>=`.

**Resolution:** The `>=` behavior is correct - we want to trigger when price reaches the buy level exactly. CLAUDE.md and SPEC.md updated to document `>=` as the intended behavior.

**Commit:** `6db079d fix: address all identified bugs from code analysis`

---

## Additional Fixes

### State Rename: HOLDING to BOUGHT [FIXED]

**Issue:** The state name `HOLDING` was confusing because `SELL_ACTIVE` also means "holding coins".

**Fix Applied:** Renamed `StateHolding` to `StateBought` throughout codebase:
- `services/grid-trading/internal/models/grid_level.go`
- `services/grid-trading/internal/repository/grid_level_repository.go`
- `services/grid-trading/internal/service/grid_service.go`
- `services/grid-trading/migrations/001_create_grid_levels.sql`
- `docs/SPEC.md`
- `CLAUDE.md`

**New State Machine:**
```
READY -> PLACING_BUY -> BUY_ACTIVE -> BOUGHT -> PLACING_SELL -> SELL_ACTIVE -> READY
```

**Commit:** `7afa478 fix: update remaining HOLDING references to BOUGHT`

---

## Bug Summary Table

| # | Original Severity | Bug | Status | Resolution |
|---|-------------------|-----|--------|------------|
| 1 | LOW | Unused price parameter | BY DESIGN | No price check needed for sell trigger |
| 2 | HIGH | GetLevelCounts inverted | FIXED | Now counts READY/BOUGHT states |
| 3 | HIGH | Symbol transformation | FIXED | Removed stripUSDT |
| 4 | MEDIUM | Division by zero risk | FIXED | Added zero check |
| 5 | MEDIUM | Missing audit trail | FIXED | Added error recording |
| 6 | LOW | Wrong comparison operator | BY DESIGN | `>=` is correct behavior |

---

## Commits

1. `6db079d fix: address all identified bugs from code analysis`
2. `73446fb docs: correct bug report - sell price check not critical`
3. `7afa478 fix: update remaining HOLDING references to BOUGHT`
4. `7a1963e fix: correct GetLevelCounts to count READY and BOUGHT states`

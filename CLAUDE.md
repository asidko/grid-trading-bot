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

## Architecture Patterns & Principles

### State and Audit Trail Consistency
When mutable state and immutable audit logs coexist, every state change must be accompanied by its audit record. Missing audit records create "ghost operations" that appear in state but have no history.

**Rule**: Any code path that modifies state MUST record the audit event atomically.

**Pattern**:
```go
// ✅ CORRECT: State + audit together
func ProcessEvent(id, data) {
    UpdateState(id, newState)
    RecordAudit(id, event, data)
}

// ❌ WRONG: State without audit
func CheckStatus(id) {
    if changed {
        UpdateState(id, newState)  // Audit missing!
    }
}
```

### Single Source of Truth (DRY for Business Logic)
When the same business event can be triggered from multiple entry points (webhooks, polling, scheduled jobs), all paths must execute identical logic. Duplication leads to subtle inconsistencies.

**Rule**: Extract shared logic into a single handler. All entry points call that handler.

**Anti-pattern**: Copy-pasting calculation or state update logic across different handlers.

### Parameter Propagation vs Hardcoding
Never hardcode or transform domain values that should be passed through the system. If a database record contains `"ETHUSDT"`, don't query external APIs with a transformed `"ETH"`.

**Rule**: Pass actual domain values explicitly through the entire call chain.

**Example**: When calling external APIs, use the exact identifier from your database, not a derived/shortened version.

### Environment Variable Handling
Docker Compose variable substitution can double-quote or split values incorrectly when both `.env` and YAML contain quotes or whitespace.

**Rule**: Choose exactly ONE place for quoting:
- Either `.env` has quotes, YAML uses bare `${VAR}`
- Or `.env` has no quotes, YAML uses `"${VAR}"`

**Debugging**: Check actual container environment with `docker inspect <container> | grep VAR`.

### External API Resilience Patterns
Third-party APIs may have undocumented limitations (data retention, rate limits, eventual consistency). Always implement fallback strategies.

**Pattern**: Fast path first, comprehensive fallback on failure:
```go
result := tryFastAPI()
if notFound {
    result = tryComprehensiveAPI()  // Slower but more complete
}
```

**Example**: Recent vs historical data endpoints, cached vs real-time lookups.

### Idempotency via State Guards
When external events can arrive multiple times (webhooks, retries, duplicates), use current state as a guard to prevent duplicate processing.

**Pattern**:
```go
func ProcessEvent(id) {
    current := FetchFreshState(id)
    if current.State != ExpectedState {
        return  // Already processed or invalid state
    }
    // Process once
}
```

**Critical**: Always fetch FRESH state from database inside the handler, not from a cached parameter.

### Complete State Machine Coverage
When polling external systems for state changes, handle ALL possible states, not just the happy path. Unhandled states cause state machine deadlock.

**Rule**: Every possible external state must map to an action:
- Success states → Progress forward
- Error states → Reset or mark failed
- Not-found states → Timeout or cleanup
- Unknown states → Log and alert

### Fail-Fast Validation at Boundaries
Validate all required inputs at API/service boundaries. Deep validation errors are harder to debug.

**Rule**: Check required parameters in the first function that receives the request:
```go
func Handler(w, r) {
    if requiredParam == "" {
        http.Error(w, "param required", 400)
        return
    }
    // Continue only with valid input
}
```

Prevents cascading errors and provides clear feedback.
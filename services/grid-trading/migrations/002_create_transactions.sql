-- Create transactions table for trade history and errors
CREATE TABLE IF NOT EXISTS transactions (
    -- Identity
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    grid_level_id INTEGER NOT NULL REFERENCES grid_levels(id),
    symbol TEXT NOT NULL,

    -- What happened (separate columns for clarity)
    side TEXT NOT NULL,           -- BUY | SELL
    status TEXT NOT NULL,         -- FILLED | ERROR

    -- Order details (NULL for errors)
    order_id TEXT,               -- Exchange order ID
    target_price TEXT NOT NULL, -- Price we aimed for
    executed_price TEXT,        -- Actual fill price (NULL if error)

    -- Amounts (NULL for errors)
    amount_coin TEXT,           -- ETH bought/sold
    amount_usdt TEXT,           -- USDT spent/received

    -- Profit tracking (only for SELL with status=FILLED)
    related_buy_id INTEGER REFERENCES transactions(id),  -- Link to original buy
    profit_usdt TEXT,           -- Sell USDT - Buy USDT
    profit_pct TEXT,             -- (profit / buy cost) * 100

    -- Error details (only when status=ERROR)
    error_code TEXT,              -- insufficient_funds, api_error, etc
    error_msg TEXT,                      -- Full error details

    -- Audit
    created_at TEXT NOT NULL DEFAULT (datetime('now')),

    -- Constraints
    CONSTRAINT check_side CHECK (side IN ('BUY', 'SELL')),
    CONSTRAINT check_status CHECK (status IN ('FILLED', 'ERROR')),
    CONSTRAINT check_filled_has_order CHECK (status = 'ERROR' OR (order_id IS NOT NULL AND executed_price IS NOT NULL)),
    CONSTRAINT check_error_has_code CHECK (status = 'FILLED' OR error_code IS NOT NULL)
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_tx_grid_level ON transactions(grid_level_id);
CREATE INDEX IF NOT EXISTS idx_tx_symbol_side_status ON transactions(symbol, side, status);
CREATE INDEX IF NOT EXISTS idx_tx_created ON transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_tx_order_id ON transactions(order_id);
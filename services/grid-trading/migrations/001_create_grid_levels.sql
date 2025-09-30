-- Create grid_levels table
CREATE TABLE IF NOT EXISTS grid_levels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    buy_price TEXT NOT NULL,
    sell_price TEXT NOT NULL,
    buy_amount TEXT NOT NULL,
    filled_amount TEXT,
    state TEXT NOT NULL DEFAULT 'READY',
    buy_order_id TEXT,
    sell_order_id TEXT,
    enabled INTEGER DEFAULT 1,
    state_changed_at TEXT NOT NULL DEFAULT (datetime('now')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),

    -- Constraints
    CONSTRAINT unique_level UNIQUE (symbol, buy_price, sell_price),
    CONSTRAINT check_prices CHECK (CAST(sell_price AS REAL) > CAST(buy_price AS REAL)),
    CONSTRAINT check_state CHECK (state IN ('READY', 'PLACING_BUY', 'BUY_ACTIVE', 'HOLDING', 'PLACING_SELL', 'SELL_ACTIVE', 'ERROR'))
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_grid_levels_symbol ON grid_levels(symbol);
CREATE INDEX IF NOT EXISTS idx_grid_levels_state ON grid_levels(state);
CREATE INDEX IF NOT EXISTS idx_grid_levels_buy_order_id ON grid_levels(buy_order_id);
CREATE INDEX IF NOT EXISTS idx_grid_levels_sell_order_id ON grid_levels(sell_order_id);
CREATE INDEX IF NOT EXISTS idx_grid_levels_enabled ON grid_levels(enabled);
CREATE INDEX IF NOT EXISTS idx_grid_levels_state_changed_at ON grid_levels(state_changed_at);
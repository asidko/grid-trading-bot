-- Create grid_levels table
CREATE TABLE IF NOT EXISTS grid_levels (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    buy_price DECIMAL(16,8) NOT NULL,
    sell_price DECIMAL(16,8) NOT NULL,
    buy_amount DECIMAL(16,8) NOT NULL,
    filled_amount DECIMAL(16,8),
    state VARCHAR(20) NOT NULL DEFAULT 'READY',
    buy_order_id VARCHAR(100),
    sell_order_id VARCHAR(100),
    enabled BOOLEAN DEFAULT true,
    state_changed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT unique_level UNIQUE (symbol, buy_price, sell_price),
    CONSTRAINT check_prices CHECK (sell_price > buy_price),
    CONSTRAINT check_state CHECK (state IN ('READY', 'PLACING_BUY', 'BUY_ACTIVE', 'HOLDING', 'PLACING_SELL', 'SELL_ACTIVE', 'ERROR'))
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_grid_levels_symbol ON grid_levels(symbol);
CREATE INDEX IF NOT EXISTS idx_grid_levels_state ON grid_levels(state);
CREATE INDEX IF NOT EXISTS idx_grid_levels_buy_order_id ON grid_levels(buy_order_id);
CREATE INDEX IF NOT EXISTS idx_grid_levels_sell_order_id ON grid_levels(sell_order_id);
CREATE INDEX IF NOT EXISTS idx_grid_levels_enabled ON grid_levels(enabled);
CREATE INDEX IF NOT EXISTS idx_grid_levels_state_changed_at ON grid_levels(state_changed_at);
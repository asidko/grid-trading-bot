-- Create transactions table for trade history and errors
CREATE TABLE IF NOT EXISTS transactions (
    -- Identity
    id SERIAL PRIMARY KEY,
    grid_level_id INTEGER NOT NULL REFERENCES grid_levels(id),
    symbol VARCHAR(20) NOT NULL,

    -- What happened (separate columns for clarity)
    side VARCHAR(10) NOT NULL,           -- BUY | SELL
    status VARCHAR(20) NOT NULL,         -- FILLED | ERROR

    -- Order details (NULL for errors)
    order_id VARCHAR(100),               -- Exchange order ID
    target_price DECIMAL(16,8) NOT NULL, -- Price we aimed for
    executed_price DECIMAL(16,8),        -- Actual fill price (NULL if error)

    -- Amounts (NULL for errors)
    amount_coin DECIMAL(16,8),           -- ETH bought/sold
    amount_usdt DECIMAL(16,8),           -- USDT spent/received

    -- Profit tracking (only for SELL with status=FILLED)
    related_buy_id INTEGER REFERENCES transactions(id),  -- Link to original buy
    profit_usdt DECIMAL(16,8),           -- Sell USDT - Buy USDT
    profit_pct DECIMAL(8,4),             -- (profit / buy cost) * 100

    -- Error details (only when status=ERROR)
    error_code VARCHAR(50),              -- insufficient_funds, api_error, etc
    error_msg TEXT,                      -- Full error details

    -- Audit
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

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
package models

import (
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type TransactionSide string
type TransactionStatus string

const (
	SideBuy  TransactionSide = "BUY"
	SideSell TransactionSide = "SELL"
)

const (
	StatusPlaced TransactionStatus = "PLACED"
	StatusFilled TransactionStatus = "FILLED"
	StatusError  TransactionStatus = "ERROR"
)

type Transaction struct {
	ID            int                  `db:"id"`
	GridLevelID   int                  `db:"grid_level_id"`
	Symbol        string               `db:"symbol"`
	Side          TransactionSide      `db:"side"`
	Status        TransactionStatus    `db:"status"`
	OrderID       sql.NullString       `db:"order_id"`
	TargetPrice   decimal.Decimal      `db:"target_price"`
	ExecutedPrice decimal.NullDecimal  `db:"executed_price"`
	AmountCoin    decimal.NullDecimal  `db:"amount_coin"`
	AmountUSDT    decimal.NullDecimal  `db:"amount_usdt"`
	RelatedBuyID  sql.NullInt64        `db:"related_buy_id"`
	ProfitUSDT    decimal.NullDecimal  `db:"profit_usdt"`
	ProfitPct     decimal.NullDecimal  `db:"profit_pct"`
	ErrorCode     sql.NullString       `db:"error_code"`
	ErrorMsg      sql.NullString       `db:"error_msg"`
	CreatedAt     time.Time            `db:"created_at"`
}
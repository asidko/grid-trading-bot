package models

import (
	"database/sql"
	"time"

	"github.com/shopspring/decimal"
)

type GridState string

const (
	StateReady       GridState = "READY"
	StatePlacingBuy  GridState = "PLACING_BUY"
	StateBuyActive   GridState = "BUY_ACTIVE"
	StateBought      GridState = "BOUGHT" // Previously HOLDING - have coins, sell order not yet placed
	StatePlacingSell GridState = "PLACING_SELL"
	StateSellActive  GridState = "SELL_ACTIVE"
	StateError       GridState = "ERROR"
)

type GridLevel struct {
	ID             int                  `db:"id"`
	Symbol         string               `db:"symbol"`
	BuyPrice       decimal.Decimal      `db:"buy_price"`
	SellPrice      decimal.Decimal      `db:"sell_price"`
	BuyAmount      decimal.Decimal      `db:"buy_amount"`
	FilledAmount   decimal.NullDecimal  `db:"filled_amount"`
	State          GridState            `db:"state"`
	BuyOrderID     sql.NullString       `db:"buy_order_id"`
	SellOrderID    sql.NullString       `db:"sell_order_id"`
	Enabled        bool                 `db:"enabled"`
	StateChangedAt time.Time            `db:"state_changed_at"`
	CreatedAt      time.Time            `db:"created_at"`
	UpdatedAt      time.Time            `db:"updated_at"`
}

func (g *GridLevel) CanPlaceBuy(currentPrice decimal.Decimal) bool {
	return g.State == StateReady &&
		g.Enabled &&
		currentPrice.GreaterThanOrEqual(g.BuyPrice) &&
		currentPrice.LessThan(g.SellPrice)
}

// CanPlaceSell checks if a sell order can be placed for this level.
// Sell orders are placed immediately after a buy fill (limit order at SellPrice),
// so no price check is needed - we just verify we have coins to sell.
func (g *GridLevel) CanPlaceSell() bool {
	return g.State == StateBought &&
		g.Enabled &&
		g.FilledAmount.Valid &&
		g.FilledAmount.Decimal.GreaterThan(decimal.Zero)
}


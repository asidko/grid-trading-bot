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
	StateHolding     GridState = "HOLDING"
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
	ErrorMsg       sql.NullString       `db:"error_msg"`
	StateChangedAt time.Time            `db:"state_changed_at"`
	CreatedAt      time.Time            `db:"created_at"`
	UpdatedAt      time.Time            `db:"updated_at"`
}

func (g *GridLevel) CanPlaceBuy(currentPrice decimal.Decimal) bool {
	return g.State == StateReady &&
		g.Enabled &&
		currentPrice.GreaterThan(g.BuyPrice)
}

func (g *GridLevel) CanPlaceSell(currentPrice decimal.Decimal) bool {
	return g.State == StateHolding &&
		g.Enabled &&
		currentPrice.LessThan(g.SellPrice) &&
		g.FilledAmount.Valid &&
		g.FilledAmount.Decimal.GreaterThan(decimal.Zero)
}

func (g *GridLevel) IsInPlacingState() bool {
	return g.State == StatePlacingBuy || g.State == StatePlacingSell
}

func (g *GridLevel) IsInActiveState() bool {
	return g.State == StateBuyActive || g.State == StateSellActive
}

func (g *GridLevel) NeedsRecovery(timeout time.Duration) bool {
	if !g.IsInPlacingState() {
		return false
	}
	return time.Since(g.StateChangedAt) > timeout
}
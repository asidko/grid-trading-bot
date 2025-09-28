package models

import (
	"github.com/shopspring/decimal"
)

type OrderSide string

const (
	SideBuy  OrderSide = "buy"
	SideSell OrderSide = "sell"
)

// OrderRequest from grid-trading service
type OrderRequest struct {
	Symbol string          `json:"symbol"`
	Price  decimal.Decimal `json:"price"`
	Side   OrderSide       `json:"side"`
	Amount decimal.Decimal `json:"amount"` // USDT for buy, coin amount for sell
}

// OrderResponse to grid-trading service
type OrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"` // "assured" means order placed on exchange
}

// OrderStatus response
type OrderStatus struct {
	OrderID      string           `json:"order_id"`
	Status       string           `json:"status"` // open, filled, cancelled
	FilledAmount *decimal.Decimal `json:"filled_amount,omitempty"`
	FillPrice    *decimal.Decimal `json:"fill_price,omitempty"`
}

// Binance order structure
type BinanceOrder struct {
	Symbol              string `json:"symbol"`
	OrderID             int64  `json:"orderId"`
	ClientOrderID       string `json:"clientOrderId"`
	Price               string `json:"price"`
	OrigQty             string `json:"origQty"`
	ExecutedQty         string `json:"executedQty"`
	CummulativeQuoteQty string `json:"cummulativeQuoteQty"`
	Status              string `json:"status"`
	Type                string `json:"type"`
	Side                string `json:"side"`
	StopPrice           string `json:"stopPrice"`
	IcebergQty          string `json:"icebergQty"`
	Time                int64  `json:"time"`
	UpdateTime          int64  `json:"updateTime"`
	IsWorking           bool   `json:"isWorking"`
}

// FillNotification to send to grid-trading service
type FillNotification struct {
	OrderID      string          `json:"order_id"`
	Symbol       string          `json:"symbol"`
	Price        decimal.Decimal `json:"price"`
	Side         string          `json:"side"`
	Status       string          `json:"status"`
	FilledAmount decimal.Decimal `json:"filled_amount"`
	FillPrice    decimal.Decimal `json:"fill_price"`
}

// ErrorNotification to send to grid-trading service
type ErrorNotification struct {
	OrderID string `json:"order_id"`
	Symbol  string `json:"symbol"`
	Side    string `json:"side"`
	Error   string `json:"error"`
}
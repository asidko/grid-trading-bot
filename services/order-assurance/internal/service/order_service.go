package service

import (
	"fmt"
	"log"
	"strconv"

	"github.com/grid-trading-bot/services/order-assurance/internal/exchange"
	"github.com/grid-trading-bot/services/order-assurance/internal/models"
	"github.com/grid-trading-bot/services/order-assurance/internal/webhook"
	"github.com/shopspring/decimal"
)

type OrderService struct {
	binance         *exchange.BinanceClient
	webhookNotifier *webhook.Notifier
}

func NewOrderService(binance *exchange.BinanceClient, webhookNotifier *webhook.Notifier) *OrderService {
	return &OrderService{
		binance:         binance,
		webhookNotifier: webhookNotifier,
	}
}

// PlaceOrder handles idempotent order placement
func (s *OrderService) PlaceOrder(req models.OrderRequest) (*models.OrderResponse, error) {
	// Convert USDT amount to coin amount for buy orders
	quantity := req.Amount
	if req.Side == models.SideBuy {
		// For buy orders, amount is in USDT, need to convert to coin quantity
		quantity = req.Amount.Div(req.Price)
	}

	// Place order on Binance (idempotent via cache)
	binanceOrder, err := s.binance.PlaceOrder(req.Symbol, req.Side, req.Price, quantity)
	if err != nil {
		return nil, fmt.Errorf("failed to place order on Binance: %w", err)
	}

	return &models.OrderResponse{
		OrderID: strconv.FormatInt(binanceOrder.OrderID, 10),
		Status:  "assured",
	}, nil
}

// GetOrderStatus retrieves current order status from Binance
func (s *OrderService) GetOrderStatus(orderID string) (*models.OrderStatus, error) {
	// Try common trading symbols
	for _, sym := range []string{"ETH", "BTC", "BNB", "SOL", "DOGE"} {
		status, err := s.fetchOrderStatus(sym, orderID)
		if err == nil && status != nil {
			return status, nil
		}
	}
	return nil, fmt.Errorf("order not found")
}

func (s *OrderService) fetchOrderStatus(symbol, orderID string) (*models.OrderStatus, error) {
	binanceOrder, err := s.binance.GetOrder(symbol, orderID)
	if err != nil {
		return nil, err
	}

	if binanceOrder == nil {
		return nil, nil
	}

	// Convert status
	status := exchange.ConvertBinanceStatus(binanceOrder.Status)

	result := &models.OrderStatus{
		OrderID: orderID,
		Status:  status,
	}

	// Add fill details if filled
	if status == "filled" {
		executedQty, _ := decimal.NewFromString(binanceOrder.ExecutedQty)
		cummulativeQuoteQty, _ := decimal.NewFromString(binanceOrder.CummulativeQuoteQty)

		// Calculate average fill price
		fillPrice := decimal.Zero
		if !executedQty.IsZero() {
			fillPrice = cummulativeQuoteQty.Div(executedQty)
		}

		result.FilledAmount = &executedQty
		result.FillPrice = &fillPrice

		// Send fill notification
		s.sendFillNotification(binanceOrder, executedQty, fillPrice)
	}

	return result, nil
}


func (s *OrderService) sendFillNotification(order *models.BinanceOrder, filledAmount, fillPrice decimal.Decimal) {
	notification := models.FillNotification{
		OrderID:      strconv.FormatInt(order.OrderID, 10),
		Symbol:       s.stripUSDT(order.Symbol),
		Price:        fillPrice,
		Side:         order.Side,
		Status:       "filled",
		FilledAmount: filledAmount,
		FillPrice:    fillPrice,
	}

	if err := s.webhookNotifier.SendFillNotification(notification); err != nil {
		log.Printf("Failed to send fill notification for order %d: %v", order.OrderID, err)
	}
}

func (s *OrderService) stripUSDT(symbol string) string {
	// Convert ETHUSDT to ETH, BTCUSDT to BTC, etc.
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4]
	}
	return symbol
}
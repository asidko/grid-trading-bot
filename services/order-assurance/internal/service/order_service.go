package service

import (
	"fmt"
	"log"
	"strconv"

	"github.com/grid-trading-bot/services/order-assurance/internal/exchange"
	"github.com/grid-trading-bot/services/order-assurance/internal/models"
	"github.com/grid-trading-bot/services/order-assurance/internal/client"
	"github.com/shopspring/decimal"
)

type OrderService struct {
	binance    *exchange.BinanceClient
	gridClient *client.Notifier
}

func NewOrderService(binance *exchange.BinanceClient, gridClient *client.Notifier) *OrderService {
	return &OrderService{
		binance:    binance,
		gridClient: gridClient,
	}
}

// PlaceOrder handles idempotent order placement
func (s *OrderService) PlaceOrder(req models.OrderRequest) (*models.OrderResponse, error) {
	// Convert USDT amount to coin amount for buy orders
	quantity := req.Amount
	if req.Side == models.SideBuy {
		// For buy orders, amount is in USDT, need to convert to coin quantity
		quantity = req.Amount.Div(req.Price)
		log.Printf("INFO: Converting buy amount - %s USDT @ %s = %s coins", req.Amount, req.Price, quantity)
	}

	log.Printf("INFO: Placing order - Symbol: %s, Side: %s, Price: %s, Quantity: %s", req.Symbol, req.Side, req.Price, quantity)

	// Place order on Binance (idempotent via cache)
	binanceOrder, err := s.binance.PlaceOrder(req.Symbol, req.Side, req.Price, quantity)
	if err != nil {
		log.Printf("ERROR: Order placement failed - Symbol: %s, Side: %s, Price: %s, Quantity: %s, Error: %v",
			req.Symbol, req.Side, req.Price, quantity, err)
		return nil, fmt.Errorf("failed to place order on Binance: %w", err)
	}

	log.Printf("SUCCESS: Order assured - Order ID: %s, Symbol: %s, Side: %s", strconv.FormatInt(binanceOrder.OrderID, 10), req.Symbol, req.Side)

	return &models.OrderResponse{
		OrderID: strconv.FormatInt(binanceOrder.OrderID, 10),
		Status:  "assured",
	}, nil
}

// GetOrderStatus retrieves current order status from Binance
func (s *OrderService) GetOrderStatus(symbol, orderID string) (*models.OrderStatus, error) {
	return s.fetchOrderStatus(symbol, orderID)
}

func (s *OrderService) fetchOrderStatus(symbol, orderID string) (*models.OrderStatus, error) {
	binanceOrder, err := s.binance.GetOrder(symbol, orderID)
	if err != nil {
		log.Printf("ERROR: Failed to fetch order status for %s: %v", orderID, err)
		return nil, err
	}

	if binanceOrder == nil {
		log.Printf("WARNING: Order %s not found on Binance", orderID)
		return nil, nil
	}

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

		log.Printf("INFO: Order %s filled - Executed: %s @ %s (Quote: %s)",
			orderID, executedQty, fillPrice, cummulativeQuoteQty)

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

	if err := s.gridClient.SendFillNotification(notification); err != nil {
		log.Printf("ERROR: Failed to send fill notification for order %d: %v", order.OrderID, err)
	} else {
		log.Printf("INFO: Sent fill notification - Order: %d, Symbol: %s, Side: %s, Amount: %s @ %s",
			order.OrderID, notification.Symbol, order.Side, filledAmount, fillPrice)
	}
}

func (s *OrderService) stripUSDT(symbol string) string {
	// Convert ETHUSDT to ETH, BTCUSDT to BTC, etc.
	if len(symbol) > 4 && symbol[len(symbol)-4:] == "USDT" {
		return symbol[:len(symbol)-4]
	}
	return symbol
}
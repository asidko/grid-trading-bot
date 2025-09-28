package service

import (
	"fmt"
	"log"
	"time"

	"github.com/grid-trading-bot/services/grid-trading/internal/client"
	"github.com/grid-trading-bot/services/grid-trading/internal/models"
	"github.com/grid-trading-bot/services/grid-trading/internal/repository"
	"github.com/shopspring/decimal"
)

type GridService struct {
	repo      *repository.GridLevelRepository
	assurance *client.OrderAssuranceClient
}

func NewGridService(repo *repository.GridLevelRepository, assurance *client.OrderAssuranceClient) *GridService {
	return &GridService{
		repo:      repo,
		assurance: assurance,
	}
}

func (s *GridService) ProcessPriceTrigger(symbol string, price decimal.Decimal) error {
	levels, err := s.repo.GetBySymbol(symbol)
	if err != nil {
		return fmt.Errorf("failed to get levels for symbol %s: %w", symbol, err)
	}

	for _, level := range levels {
		if level.CanPlaceBuy(price) {
			if err := s.tryPlaceBuyOrder(level); err != nil {
				log.Printf("Failed to place buy order for level %d: %v", level.ID, err)
			}
		} else if level.CanPlaceSell(price) {
			if err := s.tryPlaceSellOrder(level); err != nil {
				log.Printf("Failed to place sell order for level %d: %v", level.ID, err)
			}
		}
	}

	return nil
}

func (s *GridService) tryPlaceBuyOrder(level *models.GridLevel) error {
	started, err := s.repo.TryStartBuyOrder(level.ID)
	if err != nil {
		return fmt.Errorf("failed to start buy order: %w", err)
	}

	if !started {
		return nil
	}

	orderReq := client.OrderRequest{
		Symbol: level.Symbol,
		Price:  level.BuyPrice,
		Side:   client.OrderSideBuy,
		Amount: level.BuyAmount,
	}

	orderResp, err := s.assurance.PlaceOrder(orderReq)
	if err != nil {
		errMsg := err.Error()
		s.repo.UpdateState(level.ID, models.StateReady, &errMsg)
		return fmt.Errorf("failed to place buy order: %w", err)
	}

	if err := s.repo.UpdateBuyOrderPlaced(level.ID, orderResp.OrderID); err != nil {
		return fmt.Errorf("failed to update buy order placed: %w", err)
	}

	log.Printf("Placed buy order %s for level %d at price %s", orderResp.OrderID, level.ID, level.BuyPrice)
	return nil
}

func (s *GridService) tryPlaceSellOrder(level *models.GridLevel) error {
	started, err := s.repo.TryStartSellOrder(level.ID)
	if err != nil {
		return fmt.Errorf("failed to start sell order: %w", err)
	}

	if !started {
		return nil
	}

	if !level.FilledAmount.Valid {
		s.repo.UpdateState(level.ID, models.StateHolding, nil)
		return fmt.Errorf("no filled amount for level %d", level.ID)
	}

	orderReq := client.OrderRequest{
		Symbol: level.Symbol,
		Price:  level.SellPrice,
		Side:   client.OrderSideSell,
		Amount: level.FilledAmount.Decimal,
	}

	orderResp, err := s.assurance.PlaceOrder(orderReq)
	if err != nil {
		errMsg := err.Error()
		s.repo.UpdateState(level.ID, models.StateHolding, &errMsg)
		return fmt.Errorf("failed to place sell order: %w", err)
	}

	if err := s.repo.UpdateSellOrderPlaced(level.ID, orderResp.OrderID); err != nil {
		return fmt.Errorf("failed to update sell order placed: %w", err)
	}

	log.Printf("Placed sell order %s for level %d at price %s", orderResp.OrderID, level.ID, level.SellPrice)
	return nil
}

func (s *GridService) ProcessBuyFillNotification(orderID string, filledAmount decimal.Decimal) error {
	level, err := s.repo.GetByBuyOrderID(orderID)
	if err != nil {
		return fmt.Errorf("failed to get level by buy order ID: %w", err)
	}

	if level == nil {
		log.Printf("No level found for buy order %s", orderID)
		return nil
	}

	if level.State != models.StateBuyActive {
		log.Printf("Level %d not in BUY_ACTIVE state (current: %s), skipping", level.ID, level.State)
		return nil
	}

	if err := s.repo.ProcessBuyFill(level.ID, filledAmount); err != nil {
		return fmt.Errorf("failed to process buy fill: %w", err)
	}

	log.Printf("Processed buy fill for level %d, filled amount: %s", level.ID, filledAmount)
	return nil
}

func (s *GridService) ProcessSellFillNotification(orderID string) error {
	level, err := s.repo.GetBySellOrderID(orderID)
	if err != nil {
		return fmt.Errorf("failed to get level by sell order ID: %w", err)
	}

	if level == nil {
		log.Printf("No level found for sell order %s", orderID)
		return nil
	}

	if level.State != models.StateSellActive {
		log.Printf("Level %d not in SELL_ACTIVE state (current: %s), skipping", level.ID, level.State)
		return nil
	}

	if err := s.repo.ProcessSellFill(level.ID); err != nil {
		return fmt.Errorf("failed to process sell fill: %w", err)
	}

	log.Printf("Processed sell fill for level %d, cycle complete", level.ID)
	return nil
}

func (s *GridService) ProcessErrorNotification(orderID string, side string, errorMsg string) error {
	var level *models.GridLevel
	var err error

	if side == "buy" {
		level, err = s.repo.GetByBuyOrderID(orderID)
	} else if side == "sell" {
		level, err = s.repo.GetBySellOrderID(orderID)
	} else {
		return fmt.Errorf("invalid side: %s", side)
	}

	if err != nil {
		return fmt.Errorf("failed to get level by order ID: %w", err)
	}

	if level == nil {
		log.Printf("No level found for %s order %s", side, orderID)
		return nil
	}

	if err := s.repo.UpdateState(level.ID, models.StateError, &errorMsg); err != nil {
		return fmt.Errorf("failed to update state to ERROR: %w", err)
	}

	log.Printf("Level %d set to ERROR state: %s", level.ID, errorMsg)
	return nil
}

func (s *GridService) InitializeGrid(symbol string, minPrice, maxPrice, gridStep, buyAmount decimal.Decimal) error {
	currentPrice := minPrice

	for currentPrice.LessThan(maxPrice) {
		sellPrice := currentPrice.Add(gridStep)
		if sellPrice.GreaterThan(maxPrice) {
			sellPrice = maxPrice
		}

		level := &models.GridLevel{
			Symbol:    symbol,
			BuyPrice:  currentPrice,
			SellPrice: sellPrice,
			BuyAmount: buyAmount,
		}

		if err := s.repo.Create(level); err != nil {
			return fmt.Errorf("failed to create level: %w", err)
		}

		if level.ID > 0 {
			log.Printf("Created grid level %d: buy=%s, sell=%s", level.ID, currentPrice, sellPrice)
		}

		currentPrice = currentPrice.Add(gridStep)
	}

	return nil
}

func (s *GridService) SyncOrders() error {
	stuckLevels, err := s.repo.GetStuckInPlacingState(5 * time.Minute)
	if err != nil {
		return fmt.Errorf("failed to get stuck levels: %w", err)
	}

	for _, level := range stuckLevels {
		log.Printf("Processing stuck level %d in state %s", level.ID, level.State)

		if level.State == models.StatePlacingBuy {
			if level.BuyOrderID.Valid {
				s.checkAndUpdateOrderStatus(level, level.BuyOrderID.String, true)
			} else {
				// Retry order placement (idempotent)
				orderReq := client.OrderRequest{
					Symbol: level.Symbol,
					Price:  level.BuyPrice,
					Side:   client.OrderSideBuy,
					Amount: level.BuyAmount,
				}
				if orderResp, err := s.assurance.PlaceOrder(orderReq); err == nil {
					s.repo.UpdateBuyOrderPlaced(level.ID, orderResp.OrderID)
					log.Printf("Recovered buy order %s for level %d", orderResp.OrderID, level.ID)
				} else {
					s.repo.UpdateState(level.ID, models.StateReady, nil)
					log.Printf("Failed to recover buy order for level %d: %v", level.ID, err)
				}
			}
		} else if level.State == models.StatePlacingSell {
			if level.SellOrderID.Valid {
				s.checkAndUpdateOrderStatus(level, level.SellOrderID.String, false)
			} else if level.FilledAmount.Valid {
				// Retry order placement (idempotent)
				orderReq := client.OrderRequest{
					Symbol: level.Symbol,
					Price:  level.SellPrice,
					Side:   client.OrderSideSell,
					Amount: level.FilledAmount.Decimal,
				}
				if orderResp, err := s.assurance.PlaceOrder(orderReq); err == nil {
					s.repo.UpdateSellOrderPlaced(level.ID, orderResp.OrderID)
					log.Printf("Recovered sell order %s for level %d", orderResp.OrderID, level.ID)
				} else {
					s.repo.UpdateState(level.ID, models.StateHolding, nil)
					log.Printf("Failed to recover sell order for level %d: %v", level.ID, err)
				}
			} else {
				s.repo.UpdateState(level.ID, models.StateHolding, nil)
			}
		}
	}

	activeLevels, err := s.repo.GetAllActive()
	if err != nil {
		return fmt.Errorf("failed to get active levels: %w", err)
	}

	for _, level := range activeLevels {
		if level.State == models.StateBuyActive && level.BuyOrderID.Valid {
			s.checkAndUpdateOrderStatus(level, level.BuyOrderID.String, true)
		} else if level.State == models.StateSellActive && level.SellOrderID.Valid {
			s.checkAndUpdateOrderStatus(level, level.SellOrderID.String, false)
		}
	}

	return nil
}

func (s *GridService) checkAndUpdateOrderStatus(level *models.GridLevel, orderID string, isBuy bool) {
	status, err := s.assurance.GetOrderStatus(orderID)
	if err != nil {
		log.Printf("Failed to get order status for %s: %v", orderID, err)
		return
	}

	if status == nil {
		log.Printf("Order %s not found, resetting level %d", orderID, level.ID)
		if isBuy {
			s.repo.UpdateState(level.ID, models.StateReady, nil)
		} else {
			s.repo.UpdateState(level.ID, models.StateHolding, nil)
		}
		return
	}

	switch status.Status {
	case "filled":
		if isBuy && status.FilledAmount != nil {
			s.repo.ProcessBuyFill(level.ID, *status.FilledAmount)
		} else if !isBuy {
			s.repo.ProcessSellFill(level.ID)
		}
	case "cancelled":
		log.Printf("Order %s cancelled, resetting level %d", orderID, level.ID)
		if isBuy {
			s.repo.UpdateState(level.ID, models.StateReady, nil)
		} else {
			s.repo.UpdateState(level.ID, models.StateHolding, nil)
		}
	}
}
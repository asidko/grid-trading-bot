package service

import (
	"fmt"
	"log"
	"time"

	"github.com/grid-trading-bot/services/grid-trading/internal/client"
	"github.com/grid-trading-bot/services/grid-trading/internal/models"
	"github.com/shopspring/decimal"
)

// GridLevelRepositoryInterface defines the interface for grid level repository operations
// Only includes methods actually used by GridService (Interface Segregation Principle)
type GridLevelRepositoryInterface interface {
	// Query operations
	GetAll() ([]*models.GridLevel, error)
	GetBySymbol(symbol string) ([]*models.GridLevel, error)
	GetByBuyOrderID(orderID string) (*models.GridLevel, error)
	GetBySellOrderID(orderID string) (*models.GridLevel, error)
	GetStuckInPlacingState(timeout time.Duration) ([]*models.GridLevel, error)
	GetAllActive() ([]*models.GridLevel, error)

	// State management operations
	TryStartBuyOrder(id int) (bool, error)
	TryStartSellOrder(id int) (bool, error)
	UpdateState(id int, state models.GridState) error

	// Order tracking operations
	UpdateBuyOrderPlaced(id int, orderID string) error
	UpdateSellOrderPlaced(id int, orderID string) error

	// Fill processing operations
	ProcessBuyFill(id int, filledAmount decimal.Decimal) error
	ProcessSellFill(id int) error

	// Creation operations
	Create(level *models.GridLevel) error
}

// OrderAssuranceInterface defines the interface for order assurance client operations
type OrderAssuranceInterface interface {
	PlaceOrder(req client.OrderRequest) (*client.OrderResponse, error)
	GetOrderStatus(orderID string) (*client.OrderStatus, error)
}

// TransactionRepositoryInterface defines the interface for transaction repository operations
type TransactionRepositoryInterface interface {
	RecordBuyFilled(gridLevelID int, symbol string, orderID string, targetPrice, executedPrice, amountCoin, amountUSDT decimal.Decimal) error
	RecordSellFilled(gridLevelID int, symbol string, orderID string, targetPrice, executedPrice, amountCoin, amountUSDT decimal.Decimal, relatedBuyID int, profitUSDT, profitPct decimal.Decimal) error
	RecordBuyError(gridLevelID int, symbol string, targetPrice decimal.Decimal, errorCode, errorMsg string) error
	RecordSellError(gridLevelID int, symbol string, targetPrice decimal.Decimal, errorCode, errorMsg string) error
	GetLastBuyForLevel(gridLevelID int) (*models.Transaction, error)
}

type GridService struct {
	repo       GridLevelRepositoryInterface
	txRepo     TransactionRepositoryInterface
	assurance  OrderAssuranceInterface
}

// NewGridService creates a new GridService
// Accepts both concrete types and interfaces (Go's interface satisfaction is implicit)
func NewGridService(repo GridLevelRepositoryInterface, txRepo TransactionRepositoryInterface, assurance OrderAssuranceInterface) *GridService {
	return &GridService{
		repo:      repo,
		txRepo:    txRepo,
		assurance: assurance,
	}
}

// CheckHealth verifies database connectivity
func (s *GridService) CheckHealth() error {
	// Try to query the database with a simple count
	_, err := s.repo.GetAll()
	if err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}
	return nil
}

func (s *GridService) ProcessPriceTrigger(symbol string, price decimal.Decimal) error {
	levels, err := s.repo.GetBySymbol(symbol)
	if err != nil {
		return fmt.Errorf("failed to get levels for symbol %s: %w", symbol, err)
	}

	activatedCount := 0
	for _, level := range levels {
		if level.CanPlaceBuy(price) {
			log.Printf("INFO: Price %s triggered BUY level %d (target: %s)", price, level.ID, level.BuyPrice)
			if err := s.tryPlaceBuyOrder(level); err != nil {
				log.Printf("ERROR: Failed to place buy order for level %d: %v", level.ID, err)
			} else {
				activatedCount++
			}
		} else if level.CanPlaceSell(price) {
			log.Printf("INFO: Price %s triggered SELL level %d (target: %s)", price, level.ID, level.SellPrice)
			if err := s.tryPlaceSellOrder(level); err != nil {
				log.Printf("ERROR: Failed to place sell order for level %d: %v", level.ID, err)
			} else {
				activatedCount++
			}
		}
	}

	if activatedCount > 0 {
		log.Printf("INFO: Successfully activated %d orders for %s", activatedCount, symbol)
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

	log.Printf("INFO: Placing buy order for level %d - Symbol: %s, Price: %s, Amount: %s",
		level.ID, orderReq.Symbol, orderReq.Price, orderReq.Amount)

	orderResp, err := s.assurance.PlaceOrder(orderReq)
	if err != nil {
		log.Printf("ERROR: Buy order placement failed for level %d: %v", level.ID, err)
		s.repo.UpdateState(level.ID, models.StateReady)
		s.txRepo.RecordBuyError(level.ID, level.Symbol, level.BuyPrice, "order_placement_failed", err.Error())
		return fmt.Errorf("failed to place buy order: %w", err)
	}

	if err := s.repo.UpdateBuyOrderPlaced(level.ID, orderResp.OrderID); err != nil {
		log.Printf("ERROR: Failed to update database for buy order %s: %v", orderResp.OrderID, err)
		return fmt.Errorf("failed to update buy order placed: %w", err)
	}

	log.Printf("SUCCESS: Placed buy order %s for level %d at price %s, amount %s", orderResp.OrderID, level.ID, level.BuyPrice, level.BuyAmount)
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
		s.repo.UpdateState(level.ID, models.StateHolding)
		return fmt.Errorf("no filled amount for level %d", level.ID)
	}

	orderReq := client.OrderRequest{
		Symbol: level.Symbol,
		Price:  level.SellPrice,
		Side:   client.OrderSideSell,
		Amount: level.FilledAmount.Decimal,
	}

	log.Printf("INFO: Placing sell order for level %d - Symbol: %s, Price: %s, Amount: %s",
		level.ID, orderReq.Symbol, orderReq.Price, orderReq.Amount)

	orderResp, err := s.assurance.PlaceOrder(orderReq)
	if err != nil {
		log.Printf("ERROR: Sell order placement failed for level %d: %v", level.ID, err)
		s.repo.UpdateState(level.ID, models.StateHolding)
		s.txRepo.RecordSellError(level.ID, level.Symbol, level.SellPrice, "order_placement_failed", err.Error())
		return fmt.Errorf("failed to place sell order: %w", err)
	}

	if err := s.repo.UpdateSellOrderPlaced(level.ID, orderResp.OrderID); err != nil {
		return fmt.Errorf("failed to update sell order placed: %w", err)
	}

	log.Printf("Placed sell order %s for level %d at price %s", orderResp.OrderID, level.ID, level.SellPrice)
	return nil
}

func (s *GridService) ProcessBuyFillNotification(orderID string, filledAmount, fillPrice decimal.Decimal) error {
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

	// Record transaction
	amountUSDT := filledAmount.Mul(fillPrice)
	if err := s.txRepo.RecordBuyFilled(level.ID, level.Symbol, orderID, level.BuyPrice, fillPrice, filledAmount, amountUSDT); err != nil {
		log.Printf("ERROR: Failed to record buy transaction for level %d: %v", level.ID, err)
	}

	log.Printf("Processed buy fill for level %d, filled amount: %s", level.ID, filledAmount)
	return nil
}

func (s *GridService) ProcessSellFillNotification(orderID string, filledAmount, fillPrice decimal.Decimal) error {
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

	// Get the last buy transaction to calculate profit
	buyTx, err := s.txRepo.GetLastBuyForLevel(level.ID)
	if err != nil {
		log.Printf("ERROR: Failed to get last buy transaction for level %d: %v", level.ID, err)
	}
	if buyTx == nil {
		log.Printf("WARNING: No buy transaction found for level %d - cannot calculate profit", level.ID)
	}

	if err := s.repo.ProcessSellFill(level.ID); err != nil {
		return fmt.Errorf("failed to process sell fill: %w", err)
	}

	// Record transaction with profit
	sellAmountUSDT := filledAmount.Mul(fillPrice)
	var relatedBuyID int
	var profitUSDT, profitPct decimal.Decimal

	if buyTx != nil && buyTx.AmountUSDT.Valid && buyTx.AmountUSDT.Decimal.GreaterThan(decimal.Zero) {
		relatedBuyID = buyTx.ID
		profitUSDT = sellAmountUSDT.Sub(buyTx.AmountUSDT.Decimal)
		profitPct = profitUSDT.Div(buyTx.AmountUSDT.Decimal).Mul(decimal.NewFromInt(100))
		log.Printf("Processed sell fill for level %d, cycle complete. Profit: %s USDT (%s%%)", level.ID, profitUSDT, profitPct)
	} else {
		log.Printf("Processed sell fill for level %d, cycle complete. Profit: N/A (no buy transaction)", level.ID)
	}

	if err := s.txRepo.RecordSellFilled(level.ID, level.Symbol, orderID, level.SellPrice, fillPrice, filledAmount, sellAmountUSDT, relatedBuyID, profitUSDT, profitPct); err != nil {
		log.Printf("ERROR: Failed to record sell transaction for level %d: %v", level.ID, err)
	}
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

	if err := s.repo.UpdateState(level.ID, models.StateError); err != nil {
		return fmt.Errorf("failed to update state to ERROR: %w", err)
	}

	// Record error transaction
	if side == "buy" {
		s.txRepo.RecordBuyError(level.ID, level.Symbol, level.BuyPrice, "order_error", errorMsg)
	} else {
		s.txRepo.RecordSellError(level.ID, level.Symbol, level.SellPrice, "order_error", errorMsg)
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
					s.repo.UpdateState(level.ID, models.StateReady)
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
					s.repo.UpdateState(level.ID, models.StateHolding)
					log.Printf("Failed to recover sell order for level %d: %v", level.ID, err)
				}
			} else {
				s.repo.UpdateState(level.ID, models.StateHolding)
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
			s.repo.UpdateState(level.ID, models.StateReady)
		} else {
			s.repo.UpdateState(level.ID, models.StateHolding)
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
			s.repo.UpdateState(level.ID, models.StateReady)
		} else {
			s.repo.UpdateState(level.ID, models.StateHolding)
		}
	}
}

// CreateGrid creates new grid levels for a symbol, only adding missing levels (idempotent)
func (s *GridService) CreateGrid(symbol string, minPrice, maxPrice, gridStep, buyAmount decimal.Decimal) ([]*models.GridLevel, error) {
	// Calculate the number of levels
	priceRange := maxPrice.Sub(minPrice)
	numLevels := priceRange.Div(gridStep).IntPart()

	if numLevels <= 0 {
		return nil, fmt.Errorf("invalid grid parameters: no levels can be created")
	}

	// Get existing levels to check what already exists
	existingLevels, err := s.repo.GetBySymbol(symbol)
	if err != nil {
		log.Printf("Warning: failed to get existing levels for %s: %v", symbol, err)
	}

	// Create a map for quick lookup of existing levels
	existingMap := make(map[string]bool)
	for _, level := range existingLevels {
		key := fmt.Sprintf("%s-%s", level.BuyPrice.String(), level.SellPrice.String())
		existingMap[key] = true
	}

	// Create new levels
	levels := make([]*models.GridLevel, 0, int(numLevels))
	skippedCount := 0
	createdCount := 0

	for i := int64(0); i < numLevels; i++ {
		buyPrice := minPrice.Add(gridStep.Mul(decimal.NewFromInt(i)))
		sellPrice := buyPrice.Add(gridStep)

		// Skip if sell price exceeds max price
		if sellPrice.GreaterThan(maxPrice) {
			break
		}

		// Check if this level already exists
		key := fmt.Sprintf("%s-%s", buyPrice.String(), sellPrice.String())
		if existingMap[key] {
			skippedCount++
			continue
		}

		level := &models.GridLevel{
			Symbol:    symbol,
			BuyPrice:  buyPrice,
			SellPrice: sellPrice,
			BuyAmount: buyAmount,
			State:     models.StateReady,
			Enabled:   true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Insert the level
		if err := s.repo.Create(level); err != nil {
			// If it's a unique constraint violation, skip this level
			log.Printf("Failed to create level at buy=%s sell=%s: %v", buyPrice, sellPrice, err)
			continue
		}

		createdCount++
		levels = append(levels, level)
	}

	log.Printf("Grid creation for %s: created %d new levels, skipped %d existing levels", symbol, createdCount, skippedCount)
	return levels, nil
}

// GetGridLevels retrieves all grid levels for a specific symbol
func (s *GridService) GetGridLevels(symbol string) ([]*models.GridLevel, error) {
	return s.repo.GetBySymbol(symbol)
}

// GetAllGridLevels retrieves all grid levels
func (s *GridService) GetAllGridLevels() ([]*models.GridLevel, error) {
	return s.repo.GetAll()
}

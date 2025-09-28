package service

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/grid-trading-bot/services/grid-trading/internal/client"
	"github.com/grid-trading-bot/services/grid-trading/internal/models"
	"github.com/shopspring/decimal"
)

// TestGridLevel_CanPlaceBuy tests buy trigger logic with table-driven approach
func TestGridLevel_CanPlaceBuy(t *testing.T) {
	tests := []struct {
		name     string
		level    *models.GridLevel
		price    decimal.Decimal
		expected bool
		reason   string
	}{
		{
			name: "ready_state_price_above_trigger",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StateReady,
				Enabled:  true,
			},
			price:    decimal.NewFromInt(3650),
			expected: true,
			reason:   "READY state with price > buy_price should trigger",
		},
		{
			name: "ready_state_price_below_trigger",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StateReady,
				Enabled:  true,
			},
			price:    decimal.NewFromInt(3550),
			expected: false,
			reason:   "price <= buy_price should not trigger",
		},
		{
			name: "price_exactly_equal_to_trigger",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StateReady,
				Enabled:  true,
			},
			price:    decimal.NewFromInt(3600),
			expected: false,
			reason:   "price == buy_price should not trigger (requires >)",
		},
		{
			name: "disabled_level",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StateReady,
				Enabled:  false,
			},
			price:    decimal.NewFromInt(3650),
			expected: false,
			reason:   "disabled levels should never trigger",
		},
		{
			name: "holding_state",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StateHolding,
				Enabled:  true,
			},
			price:    decimal.NewFromInt(3650),
			expected: false,
			reason:   "HOLDING state should not allow buy orders",
		},
		{
			name: "placing_buy_state",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StatePlacingBuy,
				Enabled:  true,
			},
			price:    decimal.NewFromInt(3650),
			expected: false,
			reason:   "PLACING_BUY state should prevent concurrent orders",
		},
		{
			name: "buy_active_state",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StateBuyActive,
				Enabled:  true,
			},
			price:    decimal.NewFromInt(3650),
			expected: false,
			reason:   "BUY_ACTIVE state should prevent new orders",
		},
		{
			name: "sell_active_state",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StateSellActive,
				Enabled:  true,
			},
			price:    decimal.NewFromInt(3650),
			expected: false,
			reason:   "SELL_ACTIVE state should prevent new orders",
		},
		{
			name: "placing_sell_state",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StatePlacingSell,
				Enabled:  true,
			},
			price:    decimal.NewFromInt(3650),
			expected: false,
			reason:   "PLACING_SELL state should prevent new orders",
		},
		{
			name: "error_state",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromInt(3600),
				State:    models.StateError,
				Enabled:  true,
			},
			price:    decimal.NewFromInt(3650),
			expected: false,
			reason:   "ERROR state should prevent all trading",
		},
		{
			name: "high_precision_decimal_trigger",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromFloat(3599.99999999),
				State:    models.StateReady,
				Enabled:  true,
			},
			price:    decimal.NewFromFloat(3600.00000001),
			expected: true,
			reason:   "high precision decimals should work correctly",
		},
		{
			name: "high_precision_decimal_no_trigger",
			level: &models.GridLevel{
				BuyPrice: decimal.NewFromFloat(3600.00000001),
				State:    models.StateReady,
				Enabled:  true,
			},
			price:    decimal.NewFromFloat(3599.99999999),
			expected: false,
			reason:   "high precision decimals should prevent false triggers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.level.CanPlaceBuy(tt.price)
			if got != tt.expected {
				t.Errorf("CanPlaceBuy() = %v, want %v - %s", got, tt.expected, tt.reason)
			}
		})
	}
}

// TestGridLevel_CanPlaceSell tests sell trigger logic with comprehensive edge cases
func TestGridLevel_CanPlaceSell(t *testing.T) {
	tests := []struct {
		name     string
		level    *models.GridLevel
		price    decimal.Decimal
		expected bool
		reason   string
	}{
		{
			name: "holding_state_valid_trigger",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateHolding,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3850),
			expected: true,
			reason:   "HOLDING state with valid filled amount and price >= sell_price",
		},
		{
			name: "holding_state_exact_price_trigger",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateHolding,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3800),
			expected: true,
			reason:   "price == sell_price should trigger (>= condition)",
		},
		{
			name: "holding_state_price_below_trigger",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateHolding,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3750),
			expected: false,
			reason:   "price < sell_price should not trigger",
		},
		{
			name: "holding_state_invalid_filled_amount",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateHolding,
				Enabled:      true,
				FilledAmount: decimal.NullDecimal{Valid: false},
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "invalid filled_amount should prevent sell orders",
		},
		{
			name: "holding_state_zero_filled_amount",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateHolding,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.Zero),
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "zero filled_amount should prevent sell orders",
		},
		{
			name: "holding_state_negative_filled_amount",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateHolding,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromInt(-1)),
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "negative filled_amount should prevent sell orders",
		},
		{
			name: "disabled_level_with_valid_conditions",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateHolding,
				Enabled:      false,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "disabled levels should never trigger regardless of conditions",
		},
		{
			name: "ready_state_with_filled_amount",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateReady,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "READY state should not allow sell orders",
		},
		{
			name: "placing_sell_state",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StatePlacingSell,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "PLACING_SELL state should prevent concurrent orders",
		},
		{
			name: "sell_active_state",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateSellActive,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "SELL_ACTIVE state should prevent new orders",
		},
		{
			name: "buy_active_state",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateBuyActive,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "BUY_ACTIVE state should prevent sell orders",
		},
		{
			name: "placing_buy_state",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StatePlacingBuy,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "PLACING_BUY state should prevent sell orders",
		},
		{
			name: "error_state",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateError,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromInt(3850),
			expected: false,
			reason:   "ERROR state should prevent all trading",
		},
		{
			name: "very_small_filled_amount",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromInt(3800),
				State:        models.StateHolding,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.000001)),
			},
			price:    decimal.NewFromInt(3850),
			expected: true,
			reason:   "very small but positive filled_amount should allow sell",
		},
		{
			name: "high_precision_price_boundary",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromFloat(3799.99999999),
				State:        models.StateHolding,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromFloat(3799.99999999),
			expected: true,
			reason:   "exact price match with high precision should trigger",
		},
		{
			name: "high_precision_price_just_below",
			level: &models.GridLevel{
				SellPrice:    decimal.NewFromFloat(3800.00000000),
				State:        models.StateHolding,
				Enabled:      true,
				FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
			},
			price:    decimal.NewFromFloat(3799.99999999),
			expected: false,
			reason:   "price just below trigger should not activate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.level.CanPlaceSell(tt.price)
			if got != tt.expected {
				t.Errorf("CanPlaceSell() = %v, want %v - %s", got, tt.expected, tt.reason)
			}
		})
	}
}

// TestGridLevel_StateMachineValidation tests all state transitions
func TestGridLevel_StateMachineValidation(t *testing.T) {
	tests := []struct {
		name            string
		state           models.GridState
		enabled         bool
		hasFilledAmount bool
		expectBuy       bool
		expectSell      bool
	}{
		{
			name:            "ready_enabled",
			state:           models.StateReady,
			enabled:         true,
			hasFilledAmount: false,
			expectBuy:       true,
			expectSell:      false,
		},
		{
			name:            "ready_disabled",
			state:           models.StateReady,
			enabled:         false,
			hasFilledAmount: false,
			expectBuy:       false,
			expectSell:      false,
		},
		{
			name:            "holding_enabled_with_amount",
			state:           models.StateHolding,
			enabled:         true,
			hasFilledAmount: true,
			expectBuy:       false,
			expectSell:      true,
		},
		{
			name:            "holding_enabled_without_amount",
			state:           models.StateHolding,
			enabled:         true,
			hasFilledAmount: false,
			expectBuy:       false,
			expectSell:      false,
		},
		{
			name:            "placing_buy",
			state:           models.StatePlacingBuy,
			enabled:         true,
			hasFilledAmount: false,
			expectBuy:       false,
			expectSell:      false,
		},
		{
			name:            "buy_active",
			state:           models.StateBuyActive,
			enabled:         true,
			hasFilledAmount: false,
			expectBuy:       false,
			expectSell:      false,
		},
		{
			name:            "placing_sell",
			state:           models.StatePlacingSell,
			enabled:         true,
			hasFilledAmount: true,
			expectBuy:       false,
			expectSell:      false,
		},
		{
			name:            "sell_active",
			state:           models.StateSellActive,
			enabled:         true,
			hasFilledAmount: true,
			expectBuy:       false,
			expectSell:      false,
		},
		{
			name:            "error_state",
			state:           models.StateError,
			enabled:         true,
			hasFilledAmount: true,
			expectBuy:       false,
			expectSell:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := &models.GridLevel{
				BuyPrice:  decimal.NewFromInt(3600),
				SellPrice: decimal.NewFromInt(3800),
				State:     tt.state,
				Enabled:   tt.enabled,
			}

			if tt.hasFilledAmount {
				level.FilledAmount = decimal.NewNullDecimal(decimal.NewFromFloat(0.278))
			} else {
				level.FilledAmount = decimal.NullDecimal{Valid: false}
			}

			// Test buy conditions with high trigger price
			buyResult := level.CanPlaceBuy(decimal.NewFromInt(3650))
			if buyResult != tt.expectBuy {
				t.Errorf("CanPlaceBuy() = %v, want %v for state %s", buyResult, tt.expectBuy, tt.state)
			}

			// Test sell conditions with high trigger price
			sellResult := level.CanPlaceSell(decimal.NewFromInt(3850))
			if sellResult != tt.expectSell {
				t.Errorf("CanPlaceSell() = %v, want %v for state %s", sellResult, tt.expectSell, tt.state)
			}
		})
	}
}

// TestGridLevel_MultiLevelScenario tests realistic grid operation scenarios
func TestGridLevel_MultiLevelScenario(t *testing.T) {
	// Scenario: ETH price is 3500, test multiple grid levels
	currentPrice := decimal.NewFromInt(3500)

	// Level 1: Ready level below current price (should trigger buy)
	level1 := &models.GridLevel{
		ID:        1,
		BuyPrice:  decimal.NewFromInt(3400),
		SellPrice: decimal.NewFromInt(3600),
		State:     models.StateReady,
		Enabled:   true,
	}

	// Level 2: Holding level with sell price at current price (should trigger sell)
	level2 := &models.GridLevel{
		ID:           2,
		BuyPrice:     decimal.NewFromInt(3200),
		SellPrice:    decimal.NewFromInt(3400),
		State:        models.StateHolding,
		Enabled:      true,
		FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.312)),
	}

	// Level 3: Ready level above current price (should not trigger)
	level3 := &models.GridLevel{
		ID:        3,
		BuyPrice:  decimal.NewFromInt(3600),
		SellPrice: decimal.NewFromInt(3800),
		State:     models.StateReady,
		Enabled:   true,
	}

	// Level 4: Disabled level that would otherwise trigger (should not trigger)
	level4 := &models.GridLevel{
		ID:        4,
		BuyPrice:  decimal.NewFromInt(3300),
		SellPrice: decimal.NewFromInt(3500),
		State:     models.StateReady,
		Enabled:   false,
	}

	// Level 5: Holding level without filled amount (corrupted, should not trigger)
	level5 := &models.GridLevel{
		ID:           5,
		BuyPrice:     decimal.NewFromInt(3100),
		SellPrice:    decimal.NewFromInt(3300),
		State:        models.StateHolding,
		Enabled:      true,
		FilledAmount: decimal.NullDecimal{Valid: false},
	}

	levels := []*models.GridLevel{level1, level2, level3, level4, level5}

	// Test buy triggers
	buyTriggers := 0
	for _, level := range levels {
		if level.CanPlaceBuy(currentPrice) {
			buyTriggers++
		}
	}

	// Test sell triggers
	sellTriggers := 0
	for _, level := range levels {
		if level.CanPlaceSell(currentPrice) {
			sellTriggers++
		}
	}

	// Verify expected behavior
	if buyTriggers != 1 {
		t.Errorf("Expected 1 buy trigger, got %d", buyTriggers)
	}

	if sellTriggers != 1 {
		t.Errorf("Expected 1 sell trigger, got %d", sellTriggers)
	}

	// Verify specific level behaviors
	if !level1.CanPlaceBuy(currentPrice) {
		t.Error("Level 1 should trigger buy (3500 > 3400, READY, enabled)")
	}

	if !level2.CanPlaceSell(currentPrice) {
		t.Error("Level 2 should trigger sell (3500 >= 3400, HOLDING, has filled amount)")
	}

	if level3.CanPlaceBuy(currentPrice) {
		t.Error("Level 3 should NOT trigger buy (3500 < 3600)")
	}

	if level4.CanPlaceBuy(currentPrice) {
		t.Error("Level 4 should NOT trigger buy (disabled)")
	}

	if level5.CanPlaceSell(currentPrice) {
		t.Error("Level 5 should NOT trigger sell (no filled amount)")
	}
}

// Benchmark tests for performance validation
func BenchmarkGridLevel_CanPlaceBuy(b *testing.B) {
	level := &models.GridLevel{
		BuyPrice: decimal.NewFromInt(3600),
		State:    models.StateReady,
		Enabled:  true,
	}
	price := decimal.NewFromInt(3650)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		level.CanPlaceBuy(price)
	}
}

func BenchmarkGridLevel_CanPlaceSell(b *testing.B) {
	level := &models.GridLevel{
		SellPrice:    decimal.NewFromInt(3800),
		State:        models.StateHolding,
		Enabled:      true,
		FilledAmount: decimal.NewNullDecimal(decimal.NewFromFloat(0.278)),
	}
	price := decimal.NewFromInt(3850)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		level.CanPlaceSell(price)
	}
}

// Fuzz test for decimal precision edge cases
func FuzzGridLevel_DecimalPrecision(f *testing.F) {
	// Add seed cases for fuzzing
	f.Add(float64(3600.0), float64(3650.0))
	f.Add(float64(3599.99999999), float64(3600.00000001))
	f.Add(float64(3600.12345678), float64(3600.12345679))

	f.Fuzz(func(t *testing.T, buyPriceFloat, currentPriceFloat float64) {
		// Skip invalid inputs
		if buyPriceFloat <= 0 || currentPriceFloat <= 0 {
			return
		}

		buyPrice := decimal.NewFromFloat(buyPriceFloat)
		currentPrice := decimal.NewFromFloat(currentPriceFloat)

		level := &models.GridLevel{
			BuyPrice: buyPrice,
			State:    models.StateReady,
			Enabled:  true,
		}

		result := level.CanPlaceBuy(currentPrice)
		expected := currentPrice.GreaterThan(buyPrice)

		if result != expected {
			t.Errorf("CanPlaceBuy(%v) with buyPrice=%v returned %v, expected %v",
				currentPrice, buyPrice, result, expected)
		}
	})
}

// Test helpers for creating test data and mocks

// Helper to create a standard grid level
func createTestLevel(id int, symbol string, buyPrice, sellPrice int, state models.GridState, opts ...func(*models.GridLevel)) *models.GridLevel {
	level := &models.GridLevel{
		ID:        id,
		Symbol:    symbol,
		BuyPrice:  decimal.NewFromInt(int64(buyPrice)),
		SellPrice: decimal.NewFromInt(int64(sellPrice)),
		BuyAmount: decimal.NewFromFloat(1000),
		State:     state,
		Enabled:   true,
	}
	for _, opt := range opts {
		opt(level)
	}
	return level
}

// Helper to add filled amount to a level
func withFilledAmount(amount float64) func(*models.GridLevel) {
	return func(l *models.GridLevel) {
		l.FilledAmount = decimal.NewNullDecimal(decimal.NewFromFloat(amount))
	}
}

// Helper to disable a level
func disabled() func(*models.GridLevel) {
	return func(l *models.GridLevel) {
		l.Enabled = false
	}
}

// Helper to create mock service with levels
func createMockService(levels []*models.GridLevel) (*GridService, *MockGridLevelRepository, *MockOrderAssuranceClient) {
	symbolLevels := make(map[string][]*models.GridLevel)
	for _, level := range levels {
		symbolLevels[level.Symbol] = append(symbolLevels[level.Symbol], level)
	}

	mockRepo := &MockGridLevelRepository{
		levels:          levels,
		symbolLevels:    symbolLevels,
		buyOrderLevels:  make(map[string]*models.GridLevel),
		sellOrderLevels: make(map[string]*models.GridLevel),
	}

	mockClient := &MockOrderAssuranceClient{
		orderResponses: make(map[string]*client.OrderResponse),
		orderStatuses:  make(map[string]*client.OrderStatus),
	}

	return NewGridService(mockRepo, mockClient), mockRepo, mockClient
}

// Mock implementations for testing GridService flows

// Ensure our mocks implement the interfaces
var _ GridLevelRepositoryInterface = (*MockGridLevelRepository)(nil)
var _ OrderAssuranceInterface = (*MockOrderAssuranceClient)(nil)

type MockGridLevelRepository struct {
	levels                 []*models.GridLevel
	symbolLevels           map[string][]*models.GridLevel
	stuckLevels            []*models.GridLevel
	activeLevels           []*models.GridLevel
	buyOrderLevels         map[string]*models.GridLevel
	sellOrderLevels        map[string]*models.GridLevel
	lastStateUpdate        *models.GridLevel
	lastBuyOrderPlaced     *models.GridLevel
	lastSellOrderPlaced    *models.GridLevel
	shouldFailGetBySymbol  bool
	shouldFailTryStartBuy  bool
	shouldFailTryStartSell bool
}

func (m *MockGridLevelRepository) GetAll() ([]*models.GridLevel, error) {
	return m.levels, nil
}

func (m *MockGridLevelRepository) GetBySymbol(symbol string) ([]*models.GridLevel, error) {
	if m.shouldFailGetBySymbol {
		return nil, errors.New("database error")
	}
	if levels, exists := m.symbolLevels[symbol]; exists {
		return levels, nil
	}
	return []*models.GridLevel{}, nil
}

func (m *MockGridLevelRepository) GetByBuyOrderID(orderID string) (*models.GridLevel, error) {
	if level, exists := m.buyOrderLevels[orderID]; exists {
		return level, nil
	}
	return nil, nil
}

func (m *MockGridLevelRepository) GetBySellOrderID(orderID string) (*models.GridLevel, error) {
	if level, exists := m.sellOrderLevels[orderID]; exists {
		return level, nil
	}
	return nil, nil
}

func (m *MockGridLevelRepository) GetStuckInPlacingState(timeout time.Duration) ([]*models.GridLevel, error) {
	return m.stuckLevels, nil
}

func (m *MockGridLevelRepository) GetAllActive() ([]*models.GridLevel, error) {
	return m.activeLevels, nil
}

func (m *MockGridLevelRepository) TryStartBuyOrder(id int) (bool, error) {
	if m.shouldFailTryStartBuy {
		return false, errors.New("failed to start buy order")
	}
	return true, nil
}

func (m *MockGridLevelRepository) TryStartSellOrder(id int) (bool, error) {
	if m.shouldFailTryStartSell {
		return false, errors.New("failed to start sell order")
	}
	return true, nil
}

func (m *MockGridLevelRepository) UpdateState(id int, state models.GridState, errorMsg *string) error {
	for _, level := range m.levels {
		if level.ID == id {
			level.State = state
			if errorMsg != nil {
				level.ErrorMsg = sql.NullString{String: *errorMsg, Valid: true}
			}
			m.lastStateUpdate = level
			break
		}
	}
	return nil
}

func (m *MockGridLevelRepository) UpdateBuyOrderPlaced(id int, orderID string) error {
	for _, level := range m.levels {
		if level.ID == id {
			level.State = models.StateBuyActive
			level.BuyOrderID = sql.NullString{String: orderID, Valid: true}
			m.lastBuyOrderPlaced = level
			break
		}
	}
	return nil
}

func (m *MockGridLevelRepository) UpdateSellOrderPlaced(id int, orderID string) error {
	for _, level := range m.levels {
		if level.ID == id {
			level.State = models.StateSellActive
			level.SellOrderID = sql.NullString{String: orderID, Valid: true}
			m.lastSellOrderPlaced = level
			break
		}
	}
	return nil
}

func (m *MockGridLevelRepository) ProcessBuyFill(id int, filledAmount decimal.Decimal) error {
	for _, level := range m.levels {
		if level.ID == id {
			level.State = models.StateHolding
			level.FilledAmount = decimal.NewNullDecimal(filledAmount)
			level.BuyOrderID = sql.NullString{Valid: false}
			break
		}
	}
	return nil
}

func (m *MockGridLevelRepository) ProcessSellFill(id int) error {
	for _, level := range m.levels {
		if level.ID == id {
			level.State = models.StateReady
			level.FilledAmount = decimal.NullDecimal{Valid: false}
			level.SellOrderID = sql.NullString{Valid: false}
			break
		}
	}
	return nil
}

func (m *MockGridLevelRepository) Create(level *models.GridLevel) error {
	level.ID = len(m.levels) + 1
	m.levels = append(m.levels, level)
	return nil
}

type MockOrderAssuranceClient struct {
	placedOrders     []client.OrderRequest
	orderResponses   map[string]*client.OrderResponse
	orderStatuses    map[string]*client.OrderStatus
	shouldFailPlace  bool
	shouldFailStatus bool
}

func (m *MockOrderAssuranceClient) PlaceOrder(req client.OrderRequest) (*client.OrderResponse, error) {
	if m.shouldFailPlace {
		return nil, errors.New("order placement failed")
	}

	m.placedOrders = append(m.placedOrders, req)
	orderID := "order_" + req.Symbol + "_" + string(req.Side) + "_123"

	response := &client.OrderResponse{
		OrderID: orderID,
		Status:  "pending",
	}

	m.orderResponses[orderID] = response
	return response, nil
}

func (m *MockOrderAssuranceClient) GetOrderStatus(orderID string) (*client.OrderStatus, error) {
	if m.shouldFailStatus {
		return nil, errors.New("failed to get order status")
	}

	if status, exists := m.orderStatuses[orderID]; exists {
		return status, nil
	}
	return nil, nil
}

// GridService Flow Tests - Testing actual system flows and scenarios

// TestGridService_BasicFlows tests fundamental buy/sell flows
func TestGridService_BasicFlows(t *testing.T) {
	tests := []struct {
		name          string
		level         *models.GridLevel
		triggerPrice  int
		expectedSide  client.OrderSide
		shouldTrigger bool
	}{
		{
			name:          "buy_flow",
			level:         createTestLevel(1, "ETHUSDT", 3600, 3800, models.StateReady),
			triggerPrice:  3650,
			expectedSide:  client.OrderSideBuy,
			shouldTrigger: true,
		},
		{
			name:          "sell_flow",
			level:         createTestLevel(2, "ETHUSDT", 3600, 3800, models.StateHolding, withFilledAmount(0.278)),
			triggerPrice:  3850,
			expectedSide:  client.OrderSideSell,
			shouldTrigger: true,
		},
		{
			name:          "no_trigger",
			level:         createTestLevel(3, "ETHUSDT", 3800, 4000, models.StateReady),
			triggerPrice:  3650,
			shouldTrigger: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, mockClient := createMockService([]*models.GridLevel{tt.level})

			err := service.ProcessPriceTrigger("ETHUSDT", decimal.NewFromInt(int64(tt.triggerPrice)))
			if err != nil {
				t.Errorf("ProcessPriceTrigger failed: %v", err)
			}

			if tt.shouldTrigger {
				if len(mockClient.placedOrders) != 1 {
					t.Errorf("Expected 1 order, got %d", len(mockClient.placedOrders))
				}
				if mockClient.placedOrders[0].Side != tt.expectedSide {
					t.Errorf("Expected %v side, got %v", tt.expectedSide, mockClient.placedOrders[0].Side)
				}
			} else {
				if len(mockClient.placedOrders) != 0 {
					t.Errorf("Expected no orders, got %d", len(mockClient.placedOrders))
				}
			}
		})
	}
}

// TestGridService_FillNotifications tests fill processing flows
func TestGridService_FillNotifications(t *testing.T) {
	tests := []struct {
		name          string
		level         *models.GridLevel
		orderID       string
		filledAmount  decimal.Decimal
		expectedState models.GridState
		isIdempotent  bool
	}{
		{
			name:          "buy_fill",
			level:         createTestLevel(1, "ETHUSDT", 3600, 3800, models.StateBuyActive),
			orderID:       "buy_123",
			filledAmount:  decimal.NewFromFloat(0.278),
			expectedState: models.StateHolding,
		},
		{
			name:          "idempotent_buy_fill",
			level:         createTestLevel(2, "ETHUSDT", 3600, 3800, models.StateHolding, withFilledAmount(0.278)),
			orderID:       "buy_456",
			filledAmount:  decimal.NewFromFloat(0.278),
			expectedState: models.StateHolding,
			isIdempotent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockRepo, _ := createMockService([]*models.GridLevel{tt.level})
			mockRepo.buyOrderLevels[tt.orderID] = tt.level

			err := service.ProcessBuyFillNotification(tt.orderID, tt.filledAmount)
			if err != nil {
				t.Errorf("ProcessBuyFillNotification failed: %v", err)
			}

			if tt.level.State != tt.expectedState {
				t.Errorf("Expected state %s, got %s", tt.expectedState, tt.level.State)
			}
		})
	}
}

// TestGridService_CompleteGridCycle tests a complete buy->hold->sell cycle
func TestGridService_CompleteGridCycle(t *testing.T) {
	level := createTestLevel(1, "ETHUSDT", 3600, 3800, models.StateReady)
	service, mockRepo, mockClient := createMockService([]*models.GridLevel{level})

	// Step 1: Buy trigger
	service.ProcessPriceTrigger("ETHUSDT", decimal.NewFromInt(3650))
	if level.State != models.StateBuyActive || len(mockClient.placedOrders) != 1 {
		t.Fatal("Buy trigger failed")
	}

	// Step 2: Buy fill
	mockRepo.buyOrderLevels[mockRepo.lastBuyOrderPlaced.BuyOrderID.String] = level
	service.ProcessBuyFillNotification(mockRepo.lastBuyOrderPlaced.BuyOrderID.String, decimal.NewFromFloat(0.278))
	if level.State != models.StateHolding {
		t.Fatal("Buy fill failed")
	}

	// Step 3: Sell trigger
	service.ProcessPriceTrigger("ETHUSDT", decimal.NewFromInt(3850))
	if len(mockClient.placedOrders) != 2 {
		t.Fatal("Sell trigger failed")
	}

	// Step 4: Sell fill -> cycle complete
	mockRepo.sellOrderLevels[mockRepo.lastSellOrderPlaced.SellOrderID.String] = level
	service.ProcessSellFillNotification(mockRepo.lastSellOrderPlaced.SellOrderID.String)
	if level.State != models.StateReady || level.FilledAmount.Valid {
		t.Fatal("Sell fill failed - cycle incomplete")
	}
}

// TestGridService_ErrorScenarios tests various error handling flows
func TestGridService_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() (*GridService, *MockGridLevelRepository, *MockOrderAssuranceClient)
		action   func(*GridService) error
		expected string
	}{
		{
			name: "repository_failure",
			setup: func() (*GridService, *MockGridLevelRepository, *MockOrderAssuranceClient) {
				service, repo, client := createMockService([]*models.GridLevel{})
				repo.shouldFailGetBySymbol = true
				return service, repo, client
			},
			action: func(s *GridService) error {
				return s.ProcessPriceTrigger("ETHUSDT", decimal.NewFromInt(3650))
			},
			expected: "database error",
		},
		{
			name: "order_placement_failure",
			setup: func() (*GridService, *MockGridLevelRepository, *MockOrderAssuranceClient) {
				level := createTestLevel(1, "ETHUSDT", 3600, 3800, models.StateReady)
				service, repo, client := createMockService([]*models.GridLevel{level})
				client.shouldFailPlace = true
				return service, repo, client
			},
			action: func(s *GridService) error {
				return s.ProcessPriceTrigger("ETHUSDT", decimal.NewFromInt(3650))
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, _ := tt.setup()
			err := tt.action(service)

			if tt.expected != "" && (err == nil || !strings.Contains(err.Error(), tt.expected)) {
				t.Errorf("Expected error containing '%s', got %v", tt.expected, err)
			}
			if tt.expected == "" && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

// Critical System Scenarios Tests

// TestGridService_MultiLevelTriggers tests multiple levels triggering simultaneously
func TestGridService_MultiLevelTriggers(t *testing.T) {
	levels := []*models.GridLevel{
		createTestLevel(1, "ETHUSDT", 3400, 3600, models.StateReady),                            // Should trigger buy
		createTestLevel(2, "ETHUSDT", 3500, 3700, models.StateReady),                            // Should trigger buy
		createTestLevel(3, "ETHUSDT", 3200, 3400, models.StateHolding, withFilledAmount(0.294)), // Should trigger sell
		createTestLevel(4, "ETHUSDT", 3300, 3500, models.StateHolding, withFilledAmount(0.303)), // Should trigger sell
		createTestLevel(5, "ETHUSDT", 3800, 4000, models.StateReady),                            // Should NOT trigger
	}

	service, _, mockClient := createMockService(levels)
	service.ProcessPriceTrigger("ETHUSDT", decimal.NewFromInt(3600))

	// Should place 4 orders (2 buys + 2 sells), skip 1 level
	if len(mockClient.placedOrders) != 4 {
		t.Errorf("Expected 4 orders, got %d", len(mockClient.placedOrders))
	}

	buyCount, sellCount := 0, 0
	for _, order := range mockClient.placedOrders {
		if order.Side == client.OrderSideBuy {
			buyCount++
		} else {
			sellCount++
		}
	}

	if buyCount != 2 || sellCount != 2 {
		t.Errorf("Expected 2 buy + 2 sell orders, got %d buy + %d sell", buyCount, sellCount)
	}
}

// TestGridService_RealisticGridTrading tests actual grid trading scenarios
func TestGridService_RealisticGridTrading(t *testing.T) {
	// Real scenario: ETH grid trading should process all qualifying levels
	levels := []*models.GridLevel{
		createTestLevel(1, "ETHUSDT", 3600, 3800, models.StateReady),                           // Buy trigger
		createTestLevel(2, "ETHUSDT", 3400, 3750, models.StateHolding, withFilledAmount(0.25)), // Sell trigger
	}

	service, _, mockClient := createMockService(levels)

	// When: ETH price moves to $3750
	err := service.ProcessPriceTrigger("ETHUSDT", decimal.NewFromInt(3750))
	if err != nil {
		t.Fatalf("Real price trigger failed: %v", err)
	}

	// Then: Should process both triggers correctly
	if len(mockClient.placedOrders) != 2 {
		t.Errorf("Expected 2 orders, got %d", len(mockClient.placedOrders))
	}

	// Verify we have both buy and sell orders
	hasBuy, hasSell := false, false
	for _, order := range mockClient.placedOrders {
		if order.Side == client.OrderSideBuy {
			hasBuy = true
		} else {
			hasSell = true
		}
	}

	if !hasBuy || !hasSell {
		t.Error("Expected both buy and sell orders to be placed")
	}
}

// TestGridService_CrashRecovery tests the critical SyncOrders recovery after crashes
func TestGridService_CrashRecovery(t *testing.T) {
	// Real scenario: Bot crashed while placing orders, SyncOrders must recover
	stuckLevel := createTestLevel(1, "ETHUSDT", 3600, 3800, models.StatePlacingBuy)
	stuckLevel.StateChangedAt = time.Now().Add(-10 * time.Minute) // Stuck 10 min

	filledLevel := createTestLevel(2, "ETHUSDT", 3400, 3600, models.StateBuyActive)
	filledLevel.BuyOrderID = sql.NullString{String: "filled_order_123", Valid: true}

	service, mockRepo, mockClient := createMockService([]*models.GridLevel{stuckLevel, filledLevel})

	// Setup recovery scenario
	mockRepo.stuckLevels = []*models.GridLevel{stuckLevel}
	mockRepo.activeLevels = []*models.GridLevel{filledLevel}
	mockRepo.buyOrderLevels["filled_order_123"] = filledLevel
	mockClient.orderStatuses["filled_order_123"] = &client.OrderStatus{
		OrderID:      "filled_order_123",
		Status:       "filled",
		FilledAmount: &[]decimal.Decimal{decimal.NewFromFloat(0.28)}[0],
	}

	// When: SyncOrders runs (recovery mechanism)
	err := service.SyncOrders()
	if err != nil {
		t.Fatalf("Crash recovery failed: %v", err)
	}

	// Then: Should recover both levels
	if stuckLevel.State != models.StateBuyActive { // Retry should place order
		t.Error("Stuck level should be recovered")
	}
	if filledLevel.State != models.StateHolding { // Filled order should be processed
		t.Error("Filled level should be updated to HOLDING")
	}
}

// TestGridService_ExchangeRetries tests idempotent duplicate notifications
func TestGridService_ExchangeRetries(t *testing.T) {
	// Real scenario: Exchange sends duplicate fill notifications
	level := createTestLevel(1, "ETHUSDT", 3600, 3800, models.StateHolding, withFilledAmount(0.28))
	service, mockRepo, _ := createMockService([]*models.GridLevel{level})
	mockRepo.buyOrderLevels["duplicate_order"] = level

	// When: Same fill notification arrives twice (exchange retry)
	service.ProcessBuyFillNotification("duplicate_order", decimal.NewFromFloat(0.28))
	service.ProcessBuyFillNotification("duplicate_order", decimal.NewFromFloat(0.28))

	// Then: Should remain in same state (idempotent)
	if level.State != models.StateHolding {
		t.Error("Duplicate fills should be idempotent")
	}
}

// TestGridService_DatabaseFailures tests system resilience
func TestGridService_DatabaseFailures(t *testing.T) {
	service, mockRepo, _ := createMockService([]*models.GridLevel{})
	mockRepo.shouldFailGetBySymbol = true

	// When: Database fails during price processing
	err := service.ProcessPriceTrigger("ETHUSDT", decimal.NewFromInt(3750))

	// Then: Should return error without crashing
	if err == nil {
		t.Error("Database failure should return error")
	}
	if !strings.Contains(err.Error(), "database error") {
		t.Errorf("Error should indicate database failure, got: %v", err)
	}
}

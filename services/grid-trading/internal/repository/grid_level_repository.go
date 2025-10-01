package repository

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/grid-trading-bot/services/grid-trading/internal/models"
	"github.com/shopspring/decimal"
)

type GridLevelRepository struct {
	db *sql.DB
}

func NewGridLevelRepository(db *sql.DB) *GridLevelRepository {
	return &GridLevelRepository{db: db}
}

func (r *GridLevelRepository) scanLevel(scanner interface{ Scan(...interface{}) error }) (*models.GridLevel, error) {
	level := &models.GridLevel{}
	var stateChangedAt, createdAt, updatedAt string
	err := scanner.Scan(
		&level.ID, &level.Symbol, &level.BuyPrice, &level.SellPrice,
		&level.BuyAmount, &level.FilledAmount, &level.State,
		&level.BuyOrderID, &level.SellOrderID, &level.Enabled,
		&stateChangedAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse timestamps from TEXT format
	level.StateChangedAt, _ = time.Parse("2006-01-02 15:04:05", stateChangedAt)
	level.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	level.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	return level, nil
}

func (r *GridLevelRepository) GetBySymbol(symbol string) ([]*models.GridLevel, error) {
	query := `
		SELECT id, symbol, buy_price, sell_price, buy_amount, filled_amount,
		       state, buy_order_id, sell_order_id, enabled,
		       state_changed_at, created_at, updated_at
		FROM grid_levels
		WHERE symbol = $1
		ORDER BY buy_price ASC
	`

	rows, err := r.db.Query(query, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var levels []*models.GridLevel
	for rows.Next() {
		level, err := r.scanLevel(rows)
		if err != nil {
			return nil, err
		}
		levels = append(levels, level)
	}

	return levels, rows.Err()
}

func (r *GridLevelRepository) GetByID(id int) (*models.GridLevel, error) {
	query := `
		SELECT id, symbol, buy_price, sell_price, buy_amount, filled_amount,
		       state, buy_order_id, sell_order_id, enabled,
		       state_changed_at, created_at, updated_at
		FROM grid_levels
		WHERE id = $1
	`

	level, err := r.scanLevel(r.db.QueryRow(query, id))

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return level, err
}

func (r *GridLevelRepository) GetByBuyOrderID(orderID string) (*models.GridLevel, error) {
	query := `
		SELECT id, symbol, buy_price, sell_price, buy_amount, filled_amount,
		       state, buy_order_id, sell_order_id, enabled,
		       state_changed_at, created_at, updated_at
		FROM grid_levels
		WHERE buy_order_id = $1
	`

	level, err := r.scanLevel(r.db.QueryRow(query, orderID))

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return level, err
}

func (r *GridLevelRepository) GetBySellOrderID(orderID string) (*models.GridLevel, error) {
	query := `
		SELECT id, symbol, buy_price, sell_price, buy_amount, filled_amount,
		       state, buy_order_id, sell_order_id, enabled,
		       state_changed_at, created_at, updated_at
		FROM grid_levels
		WHERE sell_order_id = $1
	`

	level, err := r.scanLevel(r.db.QueryRow(query, orderID))

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return level, err
}

func (r *GridLevelRepository) GetStuckInPlacingState(timeout time.Duration) ([]*models.GridLevel, error) {
	cutoff := time.Now().Add(-timeout)
	query := `
		SELECT id, symbol, buy_price, sell_price, buy_amount, filled_amount,
		       state, buy_order_id, sell_order_id, enabled,
		       state_changed_at, created_at, updated_at
		FROM grid_levels
		WHERE state IN ('PLACING_BUY', 'PLACING_SELL')
		  AND state_changed_at < $1
	`

	rows, err := r.db.Query(query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var levels []*models.GridLevel
	for rows.Next() {
		level, err := r.scanLevel(rows)
		if err != nil {
			return nil, err
		}
		levels = append(levels, level)
	}

	return levels, rows.Err()
}

func (r *GridLevelRepository) GetAllActive() ([]*models.GridLevel, error) {
	query := `
		SELECT id, symbol, buy_price, sell_price, buy_amount, filled_amount,
		       state, buy_order_id, sell_order_id, enabled,
		       state_changed_at, created_at, updated_at
		FROM grid_levels
		WHERE state IN ('BUY_ACTIVE', 'SELL_ACTIVE')
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var levels []*models.GridLevel
	for rows.Next() {
		level, err := r.scanLevel(rows)
		if err != nil {
			return nil, err
		}
		levels = append(levels, level)
	}

	return levels, rows.Err()
}

func (r *GridLevelRepository) UpdateState(id int, state models.GridState) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, state_changed_at = datetime('now'), updated_at = datetime('now')
		WHERE id = $2
	`

	result, err := tx.Exec(query, state, id)
	if err != nil {
		log.Printf("ERROR: Failed to update state for level %d to %s: %v", id, state, err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if err := tx.Commit(); err != nil {
		log.Printf("ERROR: Failed to commit state update for level %d: %v", id, err)
		return err
	}

	if rowsAffected > 0 {
		log.Printf("INFO: Level %d state → %s", id, state)
	} else {
		log.Printf("WARNING: Level %d state update to %s affected 0 rows", id, state)
	}

	return nil
}

func (r *GridLevelRepository) UpdateBuyOrderPlaced(id int, orderID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, buy_order_id = $2, state_changed_at = datetime('now'), updated_at = datetime('now')
		WHERE id = $3 AND state = $4
	`

	result, err := tx.Exec(query, models.StateBuyActive, orderID, id, models.StatePlacingBuy)
	if err != nil {
		log.Printf("ERROR: Failed to update buy order for level %d: %v", id, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		log.Printf("ERROR: Level %d not in PLACING_BUY state, cannot update buy order %s", id, orderID)
		return fmt.Errorf("level %d not in PLACING_BUY state", id)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR: Failed to commit buy order update for level %d: %v", id, err)
		return err
	}

	log.Printf("INFO: Level %d → BUY_ACTIVE, buy_order_id=%s", id, orderID)
	return nil
}

func (r *GridLevelRepository) UpdateSellOrderPlaced(id int, orderID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, sell_order_id = $2, state_changed_at = datetime('now'), updated_at = datetime('now')
		WHERE id = $3 AND state = $4
	`

	result, err := tx.Exec(query, models.StateSellActive, orderID, id, models.StatePlacingSell)
	if err != nil {
		log.Printf("ERROR: Failed to update sell order for level %d: %v", id, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		log.Printf("ERROR: Level %d not in PLACING_SELL state, cannot update sell order %s", id, orderID)
		return fmt.Errorf("level %d not in PLACING_SELL state", id)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR: Failed to commit sell order update for level %d: %v", id, err)
		return err
	}

	log.Printf("INFO: Level %d → SELL_ACTIVE, sell_order_id=%s", id, orderID)
	return nil
}

func (r *GridLevelRepository) ProcessBuyFill(id int, filledAmount decimal.Decimal) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, filled_amount = $2,
		    state_changed_at = datetime('now'), updated_at = datetime('now')
		WHERE id = $3 AND state = $4
	`

	result, err := tx.Exec(query, models.StateHolding, filledAmount, id, models.StateBuyActive)
	if err != nil {
		log.Printf("ERROR: Failed to process buy fill for level %d: %v", id, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		log.Printf("WARNING: Level %d not in BUY_ACTIVE state, skipping buy fill processing", id)
		return nil
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR: Failed to commit buy fill for level %d: %v", id, err)
		return err
	}

	log.Printf("INFO: Level %d → HOLDING, filled_amount=%s", id, filledAmount)
	return nil
}

func (r *GridLevelRepository) ProcessSellFill(id int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, filled_amount = NULL, sell_order_id = NULL,
		    state_changed_at = datetime('now'), updated_at = datetime('now')
		WHERE id = $2 AND state = $3
	`

	result, err := tx.Exec(query, models.StateReady, id, models.StateSellActive)
	if err != nil {
		log.Printf("ERROR: Failed to process sell fill for level %d: %v", id, err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		log.Printf("WARNING: Level %d not in SELL_ACTIVE state, skipping sell fill processing", id)
		return nil
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR: Failed to commit sell fill for level %d: %v", id, err)
		return err
	}

	log.Printf("INFO: Level %d → READY (cycle complete), cleared filled_amount and sell_order_id", id)
	return nil
}

func (r *GridLevelRepository) TryStartBuyOrder(id int) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, state_changed_at = datetime('now'), updated_at = datetime('now')
		WHERE id = $2 AND state = $3 AND enabled = true
	`

	result, err := tx.Exec(query, models.StatePlacingBuy, id, models.StateReady)
	if err != nil {
		log.Printf("ERROR: Failed to try start buy order for level %d: %v", id, err)
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected == 0 {
		return false, nil
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR: Failed to commit start buy order for level %d: %v", id, err)
		return false, err
	}

	log.Printf("INFO: Level %d → PLACING_BUY", id)
	return true, nil
}

func (r *GridLevelRepository) TryStartSellOrder(id int) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, state_changed_at = datetime('now'), updated_at = datetime('now')
		WHERE id = $2 AND state = $3 AND enabled = true AND filled_amount IS NOT NULL
	`

	result, err := tx.Exec(query, models.StatePlacingSell, id, models.StateHolding)
	if err != nil {
		log.Printf("ERROR: Failed to try start sell order for level %d: %v", id, err)
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected == 0 {
		return false, nil
	}

	if err := tx.Commit(); err != nil {
		log.Printf("ERROR: Failed to commit start sell order for level %d: %v", id, err)
		return false, err
	}

	log.Printf("INFO: Level %d → PLACING_SELL", id)
	return true, nil
}

func (r *GridLevelRepository) Create(level *models.GridLevel) error {
	query := `
		INSERT INTO grid_levels (
			symbol, buy_price, sell_price, buy_amount, state, enabled
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (symbol, buy_price, sell_price) DO NOTHING
		RETURNING id
	`

	err := r.db.QueryRow(
		query,
		level.Symbol,
		level.BuyPrice,
		level.SellPrice,
		level.BuyAmount,
		models.StateReady,
		true,
	).Scan(&level.ID)

	if err == sql.ErrNoRows {
		return nil
	}

	return err
}

// GetAll retrieves all grid levels
func (r *GridLevelRepository) GetAll() ([]*models.GridLevel, error) {
	query := `
		SELECT id, symbol, buy_price, sell_price, buy_amount, filled_amount,
		       state, buy_order_id, sell_order_id, enabled,
		       state_changed_at, created_at, updated_at
		FROM grid_levels
		ORDER BY symbol, buy_price ASC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var levels []*models.GridLevel
	for rows.Next() {
		level, err := r.scanLevel(rows)
		if err != nil {
			return nil, err
		}
		levels = append(levels, level)
	}

	return levels, rows.Err()
}

// GetDistinctSymbols retrieves all unique symbols used in grid levels
func (r *GridLevelRepository) GetDistinctSymbols() ([]string, error) {
	query := `
		SELECT DISTINCT symbol
		FROM grid_levels
		ORDER BY symbol
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		symbols = append(symbols, symbol)
	}

	return symbols, rows.Err()
}

func (r *GridLevelRepository) GetLevelCounts() (holding, ready int, err error) {
	query := `
		SELECT
			COUNT(CASE WHEN state = 'HOLDING' THEN 1 END) as holding,
			COUNT(CASE WHEN state = 'READY' THEN 1 END) as ready
		FROM grid_levels
		WHERE enabled = 1
	`

	err = r.db.QueryRow(query).Scan(&holding, &ready)
	return holding, ready, err
}


package repository

import (
	"database/sql"
	"fmt"
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
	err := scanner.Scan(
		&level.ID, &level.Symbol, &level.BuyPrice, &level.SellPrice,
		&level.BuyAmount, &level.FilledAmount, &level.State,
		&level.BuyOrderID, &level.SellOrderID, &level.Enabled,
		&level.StateChangedAt, &level.CreatedAt, &level.UpdatedAt,
	)
	return level, err
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
		SET state = $1, state_changed_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`

	_, err = tx.Exec(query, state, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *GridLevelRepository) UpdateBuyOrderPlaced(id int, orderID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, buy_order_id = $2, state_changed_at = NOW(), updated_at = NOW()
		WHERE id = $3 AND state = $4
	`

	result, err := tx.Exec(query, models.StateBuyActive, orderID, id, models.StatePlacingBuy)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("level %d not in PLACING_BUY state", id)
	}

	return tx.Commit()
}

func (r *GridLevelRepository) UpdateSellOrderPlaced(id int, orderID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, sell_order_id = $2, state_changed_at = NOW(), updated_at = NOW()
		WHERE id = $3 AND state = $4
	`

	result, err := tx.Exec(query, models.StateSellActive, orderID, id, models.StatePlacingSell)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("level %d not in PLACING_SELL state", id)
	}

	return tx.Commit()
}

func (r *GridLevelRepository) ProcessBuyFill(id int, filledAmount decimal.Decimal) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, filled_amount = $2, buy_order_id = NULL,
		    state_changed_at = NOW(), updated_at = NOW()
		WHERE id = $3 AND state = $4
	`

	result, err := tx.Exec(query, models.StateHolding, filledAmount, id, models.StateBuyActive)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return nil
	}

	return tx.Commit()
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
		    state_changed_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND state = $3
	`

	result, err := tx.Exec(query, models.StateReady, id, models.StateSellActive)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return nil
	}

	return tx.Commit()
}

func (r *GridLevelRepository) TryStartBuyOrder(id int) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, state_changed_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND state = $3 AND enabled = true
	`

	result, err := tx.Exec(query, models.StatePlacingBuy, id, models.StateReady)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected == 0 {
		return false, nil
	}

	return true, tx.Commit()
}

func (r *GridLevelRepository) TryStartSellOrder(id int) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	query := `
		UPDATE grid_levels
		SET state = $1, state_changed_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND state = $3 AND enabled = true AND filled_amount IS NOT NULL
	`

	result, err := tx.Exec(query, models.StatePlacingSell, id, models.StateHolding)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected == 0 {
		return false, nil
	}

	return true, tx.Commit()
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


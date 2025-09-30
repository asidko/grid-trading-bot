package repository

import (
	"database/sql"

	"github.com/grid-trading-bot/services/grid-trading/internal/models"
	"github.com/shopspring/decimal"
)

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) RecordBuyFilled(
	gridLevelID int,
	symbol string,
	orderID string,
	targetPrice decimal.Decimal,
	executedPrice decimal.Decimal,
	amountCoin decimal.Decimal,
	amountUSDT decimal.Decimal,
) error {
	query := `
		INSERT INTO transactions (
			grid_level_id, symbol, side, status,
			order_id, target_price, executed_price,
			amount_coin, amount_usdt
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	var txID int
	err := r.db.QueryRow(
		query,
		gridLevelID,
		symbol,
		models.SideBuy,
		models.StatusFilled,
		orderID,
		targetPrice,
		executedPrice,
		amountCoin,
		amountUSDT,
	).Scan(&txID)

	return err
}

func (r *TransactionRepository) RecordSellFilled(
	gridLevelID int,
	symbol string,
	orderID string,
	targetPrice decimal.Decimal,
	executedPrice decimal.Decimal,
	amountCoin decimal.Decimal,
	amountUSDT decimal.Decimal,
	relatedBuyID int,
	profitUSDT decimal.Decimal,
	profitPct decimal.Decimal,
) error {
	query := `
		INSERT INTO transactions (
			grid_level_id, symbol, side, status,
			order_id, target_price, executed_price,
			amount_coin, amount_usdt,
			related_buy_id, profit_usdt, profit_pct
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.Exec(
		query,
		gridLevelID,
		symbol,
		models.SideSell,
		models.StatusFilled,
		orderID,
		targetPrice,
		executedPrice,
		amountCoin,
		amountUSDT,
		relatedBuyID,
		profitUSDT,
		profitPct,
	)

	return err
}

func (r *TransactionRepository) RecordBuyError(
	gridLevelID int,
	symbol string,
	targetPrice decimal.Decimal,
	errorCode string,
	errorMsg string,
) error {
	return r.recordError(gridLevelID, symbol, targetPrice, errorCode, errorMsg, string(models.SideBuy))
}

func (r *TransactionRepository) RecordSellError(
	gridLevelID int,
	symbol string,
	targetPrice decimal.Decimal,
	errorCode string,
	errorMsg string,
) error {
	return r.recordError(gridLevelID, symbol, targetPrice, errorCode, errorMsg, string(models.SideSell))
}

func (r *TransactionRepository) recordError(
	gridLevelID int,
	symbol string,
	targetPrice decimal.Decimal,
	errorCode string,
	errorMsg string,
	side string,
) error {
	query := `
		INSERT INTO transactions (
			grid_level_id, symbol, side, status,
			target_price, error_code, error_msg
		)
		SELECT $1, $2, $3, $4, $5, $6, $7
		WHERE NOT EXISTS (
			SELECT 1 FROM transactions
			WHERE grid_level_id = $1
			  AND symbol = $2
			  AND side = $3
			  AND status = $4
			  AND target_price = $5
			  AND error_msg = $7
			  AND created_at > NOW() - INTERVAL '1 hour'
		)
	`

	_, err := r.db.Exec(
		query,
		gridLevelID,
		symbol,
		side,
		models.StatusError,
		targetPrice,
		errorCode,
		errorMsg,
	)

	return err
}

func (r *TransactionRepository) GetLastBuyForLevel(gridLevelID int) (*models.Transaction, error) {
	query := `
		SELECT id, grid_level_id, symbol, side, status,
		       order_id, target_price, executed_price,
		       amount_coin, amount_usdt,
		       related_buy_id, profit_usdt, profit_pct,
		       error_code, error_msg, created_at
		FROM transactions
		WHERE grid_level_id = $1 AND side = $2 AND status = $3
		ORDER BY created_at DESC
		LIMIT 1
	`

	tx := &models.Transaction{}
	err := r.db.QueryRow(query, gridLevelID, models.SideBuy, models.StatusFilled).Scan(
		&tx.ID, &tx.GridLevelID, &tx.Symbol, &tx.Side, &tx.Status,
		&tx.OrderID, &tx.TargetPrice, &tx.ExecutedPrice,
		&tx.AmountCoin, &tx.AmountUSDT,
		&tx.RelatedBuyID, &tx.ProfitUSDT, &tx.ProfitPct,
		&tx.ErrorCode, &tx.ErrorMsg, &tx.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	return tx, err
}
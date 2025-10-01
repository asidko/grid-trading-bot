package repository

import (
	"database/sql"
	"log"
	"time"

	"github.com/grid-trading-bot/services/grid-trading/internal/models"
	"github.com/shopspring/decimal"
)

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) RecordBuyPlaced(
	gridLevelID int,
	symbol string,
	orderID string,
	targetPrice decimal.Decimal,
	amountUSDT decimal.Decimal,
) error {
	query := `
		INSERT INTO transactions (
			grid_level_id, symbol, side, status,
			order_id, target_price, amount_usdt
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Exec(
		query,
		gridLevelID,
		symbol,
		models.SideBuy,
		models.StatusPlaced,
		orderID,
		targetPrice,
		amountUSDT,
	)

	if err != nil {
		log.Printf("ERROR: Failed to record BUY PLACED transaction for level %d: %v", gridLevelID, err)
	} else {
		log.Printf("INFO: Recorded BUY PLACED - Level: %d, Order: %s, Target: %s, Amount: %s USDT", gridLevelID, orderID, targetPrice, amountUSDT)
	}

	return err
}

func (r *TransactionRepository) RecordSellPlaced(
	gridLevelID int,
	symbol string,
	orderID string,
	targetPrice decimal.Decimal,
	amountCoin decimal.Decimal,
) error {
	query := `
		INSERT INTO transactions (
			grid_level_id, symbol, side, status,
			order_id, target_price, amount_coin
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Exec(
		query,
		gridLevelID,
		symbol,
		models.SideSell,
		models.StatusPlaced,
		orderID,
		targetPrice,
		amountCoin,
	)

	if err != nil {
		log.Printf("ERROR: Failed to record SELL PLACED transaction for level %d: %v", gridLevelID, err)
	} else {
		log.Printf("INFO: Recorded SELL PLACED - Level: %d, Order: %s, Target: %s, Amount: %s coins", gridLevelID, orderID, targetPrice, amountCoin)
	}

	return err
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

	if err != nil {
		log.Printf("ERROR: Failed to record BUY FILLED transaction for level %d: %v", gridLevelID, err)
	} else {
		log.Printf("INFO: Recorded BUY FILLED (tx %d) - Level: %d, Order: %s, Executed: %s (target: %s), Amount: %s coins = %s USDT",
			txID, gridLevelID, orderID, executedPrice, targetPrice, amountCoin, amountUSDT)
	}

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
		RETURNING id
	`

	var txID int
	err := r.db.QueryRow(
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
	).Scan(&txID)

	if err != nil {
		log.Printf("ERROR: Failed to record SELL FILLED transaction for level %d: %v", gridLevelID, err)
	} else {
		if relatedBuyID > 0 {
			log.Printf("INFO: Recorded SELL FILLED (tx %d) - Level: %d, Order: %s, Executed: %s (target: %s), Amount: %s coins = %s USDT, Related Buy: %d, Profit: %s USDT (%s%%)",
				txID, gridLevelID, orderID, executedPrice, targetPrice, amountCoin, amountUSDT, relatedBuyID, profitUSDT, profitPct)
		} else {
			log.Printf("INFO: Recorded SELL FILLED (tx %d) - Level: %d, Order: %s, Executed: %s (target: %s), Amount: %s coins = %s USDT (no related buy)",
				txID, gridLevelID, orderID, executedPrice, targetPrice, amountCoin, amountUSDT)
		}
	}

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
			  AND created_at > datetime('now', '-1 hour')
		)
	`

	result, err := r.db.Exec(
		query,
		gridLevelID,
		symbol,
		side,
		models.StatusError,
		targetPrice,
		errorCode,
		errorMsg,
	)

	if err != nil {
		log.Printf("ERROR: Failed to record %s ERROR transaction for level %d: %v", side, gridLevelID, err)
	} else {
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			log.Printf("WARNING: Recorded %s ERROR - Level: %d, Target: %s, Code: %s, Message: %s",
				side, gridLevelID, targetPrice, errorCode, errorMsg)
		} else {
			log.Printf("DEBUG: Duplicate %s ERROR for level %d within 1 hour, skipped recording", side, gridLevelID)
		}
	}

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
	var createdAtStr string
	err := r.db.QueryRow(query, gridLevelID, models.SideBuy, models.StatusFilled).Scan(
		&tx.ID, &tx.GridLevelID, &tx.Symbol, &tx.Side, &tx.Status,
		&tx.OrderID, &tx.TargetPrice, &tx.ExecutedPrice,
		&tx.AmountCoin, &tx.AmountUSDT,
		&tx.RelatedBuyID, &tx.ProfitUSDT, &tx.ProfitPct,
		&tx.ErrorCode, &tx.ErrorMsg, &createdAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	tx.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	return tx, nil
}

func (r *TransactionRepository) GetDailyStats() (buys, sells, errors int, profit decimal.Decimal, err error) {
	query := `
		SELECT
			COUNT(CASE WHEN side = 'BUY' AND status = 'FILLED' THEN 1 END) as buys_today,
			COUNT(CASE WHEN side = 'SELL' AND status = 'FILLED' THEN 1 END) as sells_today,
			COUNT(CASE WHEN status = 'ERROR' THEN 1 END) as errors_today,
			COALESCE(SUM(CASE WHEN side = 'SELL' AND status = 'FILLED' THEN profit_usdt ELSE 0 END), 0) as profit_today
		FROM transactions
		WHERE date(created_at) = date('now')
	`

	var profitStr string
	err = r.db.QueryRow(query).Scan(&buys, &sells, &errors, &profitStr)
	if err != nil {
		return 0, 0, 0, decimal.Zero, err
	}

	profit, err = decimal.NewFromString(profitStr)
	if err != nil {
		profit = decimal.Zero
	}

	return buys, sells, errors, profit, nil
}

func (r *TransactionRepository) GetProfitStats() (today, week, month, allTime decimal.Decimal, err error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN date(created_at) = date('now') THEN profit_usdt ELSE 0 END), 0) as profit_today,
			COALESCE(SUM(CASE WHEN created_at >= date('now', 'weekday 0', '-6 days') THEN profit_usdt ELSE 0 END), 0) as profit_week,
			COALESCE(SUM(CASE WHEN strftime('%Y-%m', created_at) = strftime('%Y-%m', 'now') THEN profit_usdt ELSE 0 END), 0) as profit_month,
			COALESCE(SUM(profit_usdt), 0) as profit_all_time
		FROM transactions
		WHERE side = 'SELL' AND status = 'FILLED'
	`

	var todayStr, weekStr, monthStr, allTimeStr string
	err = r.db.QueryRow(query).Scan(&todayStr, &weekStr, &monthStr, &allTimeStr)
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, err
	}

	today, _ = decimal.NewFromString(todayStr)
	week, _ = decimal.NewFromString(weekStr)
	month, _ = decimal.NewFromString(monthStr)
	allTime, _ = decimal.NewFromString(allTimeStr)

	return today, week, month, allTime, nil
}

func (r *TransactionRepository) GetLastBuy() (*models.Transaction, error) {
	query := `
		SELECT id, grid_level_id, symbol, side, status,
		       order_id, target_price, executed_price,
		       amount_coin, amount_usdt,
		       related_buy_id, profit_usdt, profit_pct,
		       error_code, error_msg, created_at
		FROM transactions
		WHERE side = 'BUY' AND status = 'FILLED'
		ORDER BY created_at DESC
		LIMIT 1
	`

	tx := &models.Transaction{}
	var createdAtStr string
	err := r.db.QueryRow(query).Scan(
		&tx.ID, &tx.GridLevelID, &tx.Symbol, &tx.Side, &tx.Status,
		&tx.OrderID, &tx.TargetPrice, &tx.ExecutedPrice,
		&tx.AmountCoin, &tx.AmountUSDT,
		&tx.RelatedBuyID, &tx.ProfitUSDT, &tx.ProfitPct,
		&tx.ErrorCode, &tx.ErrorMsg, &createdAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	tx.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	return tx, nil
}

func (r *TransactionRepository) GetLastSell() (*models.Transaction, error) {
	query := `
		SELECT id, grid_level_id, symbol, side, status,
		       order_id, target_price, executed_price,
		       amount_coin, amount_usdt,
		       related_buy_id, profit_usdt, profit_pct,
		       error_code, error_msg, created_at
		FROM transactions
		WHERE side = 'SELL' AND status = 'FILLED'
		ORDER BY created_at DESC
		LIMIT 1
	`

	tx := &models.Transaction{}
	var createdAtStr string
	err := r.db.QueryRow(query).Scan(
		&tx.ID, &tx.GridLevelID, &tx.Symbol, &tx.Side, &tx.Status,
		&tx.OrderID, &tx.TargetPrice, &tx.ExecutedPrice,
		&tx.AmountCoin, &tx.AmountUSDT,
		&tx.RelatedBuyID, &tx.ProfitUSDT, &tx.ProfitPct,
		&tx.ErrorCode, &tx.ErrorMsg, &createdAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	tx.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	return tx, nil
}
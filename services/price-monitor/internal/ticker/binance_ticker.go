package ticker

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	BinanceAPIURL = "https://api.binance.com"
)

type PriceUpdate struct {
	Symbol string
	Price  decimal.Decimal
}

type BinanceTicker struct {
	client  *http.Client
	baseURL string
}

func NewBinanceTicker() *BinanceTicker {
	return &BinanceTicker{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: BinanceAPIURL,
	}
}

// GetPrices fetches current prices for multiple symbols
func (bt *BinanceTicker) GetPrices(symbols []string) (map[string]decimal.Decimal, error) {
	// Normalize symbols to uppercase
	normalizedSymbols := make([]string, len(symbols))
	for i, symbol := range symbols {
		normalizedSymbols[i] = strings.ToUpper(symbol)
	}

	// Use json.Marshal for proper JSON encoding
	symbolsJSON, err := json.Marshal(normalizedSymbols)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal symbols: %w", err)
	}

	// Build URL with proper URL encoding
	reqURL := fmt.Sprintf("%s/api/v3/ticker/price?symbols=%s",
		bt.baseURL,
		string(symbolsJSON))

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := bt.client.Do(req)
	if err != nil {
		log.Printf("ERROR: Failed to fetch prices from Binance: %v", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: Binance API error %d: %s", resp.StatusCode, body)
		return nil, fmt.Errorf("binance API error %d: %s", resp.StatusCode, body)
	}

	// Parse response
	var tickers []struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}

	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Build result map
	result := make(map[string]decimal.Decimal)
	for _, ticker := range tickers {
		price, err := decimal.NewFromString(ticker.Price)
		if err != nil {
			log.Printf("WARNING: Invalid price for %s: %s (error: %v)", ticker.Symbol, ticker.Price, err)
			continue
		}

		result[ticker.Symbol] = price
	}

	return result, nil
}

// GetPrice fetches current price for a single symbol
func (bt *BinanceTicker) GetPrice(symbol string) (decimal.Decimal, error) {
	prices, err := bt.GetPrices([]string{symbol})
	if err != nil {
		return decimal.Zero, err
	}

	price, ok := prices[symbol]
	if !ok {
		return decimal.Zero, fmt.Errorf("price not found for symbol %s", symbol)
	}

	return price, nil
}
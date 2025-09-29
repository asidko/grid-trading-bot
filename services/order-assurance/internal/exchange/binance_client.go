package exchange

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grid-trading-bot/services/order-assurance/internal/models"
	"github.com/shopspring/decimal"
)

const (
	BinanceAPIURL = "https://api.binance.com"
)

// SymbolInfo contains trading rules for a symbol
type SymbolInfo struct {
	MinQty   decimal.Decimal // Minimum order quantity
	MaxQty   decimal.Decimal // Maximum order quantity
	StepSize decimal.Decimal // Quantity step size
	MinPrice decimal.Decimal // Minimum price
	MaxPrice decimal.Decimal // Maximum price
	TickSize decimal.Decimal // Price tick size
	MinNotional decimal.Decimal // Minimum notional value (price * quantity)
}

type BinanceClient struct {
	apiKey    string
	apiSecret string
	baseURL   string
	client    *http.Client

	// Cache for open orders to implement idempotency
	orderCache      map[string]*models.BinanceOrder
	orderCacheMutex sync.RWMutex
	cacheExpiry     time.Duration
	lastCacheUpdate time.Time

	// Symbol restrictions cache
	symbolInfo      map[string]*SymbolInfo
	symbolInfoMutex sync.RWMutex
	symbolInfoTime  time.Time
}

func NewBinanceClient(apiKey, apiSecret string) *BinanceClient {
	return &BinanceClient{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   BinanceAPIURL,
		client:    &http.Client{Timeout: 10 * time.Second},
		orderCache: make(map[string]*models.BinanceOrder),
		cacheExpiry: 5 * time.Second, // Short cache for idempotency
		symbolInfo: make(map[string]*SymbolInfo),
	}
}

// PlaceOrder places a LIMIT order on Binance
func (bc *BinanceClient) PlaceOrder(symbol string, side models.OrderSide, price, quantity decimal.Decimal) (*models.BinanceOrder, error) {
	// Ensure we have symbol info
	symbolPair := bc.formatSymbol(symbol)
	info, err := bc.getSymbolInfo(symbolPair)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbol info: %w", err)
	}

	// Apply symbol restrictions
	price = bc.roundToTickSize(price, info.TickSize)
	quantity = bc.roundToStepSize(quantity, info.StepSize)

	originalQuantity := quantity

	// Adjust quantity to meet minimum notional if needed
	notional := price.Mul(quantity)
	if notional.LessThan(info.MinNotional) {
		// Calculate minimum quantity needed to meet notional requirement
		// Add a small buffer (1%) to ensure we meet the requirement
		minQuantityNeeded := info.MinNotional.Mul(decimal.NewFromFloat(1.01)).Div(price)
		// Round up to step size
		quantity = bc.roundUpToStepSize(minQuantityNeeded, info.StepSize)
		notional = price.Mul(quantity)
		log.Printf("Adjusted quantity from %s to %s to meet minimum notional of %s",
			originalQuantity, quantity, info.MinNotional)
	}

	// Adjust for minimum quantity restriction
	if quantity.LessThan(info.MinQty) {
		quantity = info.MinQty
		log.Printf("Adjusted quantity from %s to %s to meet minimum quantity requirement",
			originalQuantity, quantity)
	}

	// Check maximum quantity restriction (this one we can't adjust)
	if quantity.GreaterThan(info.MaxQty) {
		return nil, fmt.Errorf("required quantity %s exceeds maximum allowed %s", quantity, info.MaxQty)
	}

	// Check cache for idempotency
	cacheKey := bc.createCacheKey(symbol, side, price, quantity)
	if existingOrder := bc.getFromCache(cacheKey); existingOrder != nil {
		currentOrder, err := bc.GetOrder(symbol, strconv.FormatInt(existingOrder.OrderID, 10))
		if err == nil && currentOrder != nil && (currentOrder.Status == "NEW" || currentOrder.Status == "PARTIALLY_FILLED") {
			return currentOrder, nil
		}
	}

	params := url.Values{}
	params.Set("symbol", symbolPair)
	params.Set("side", strings.ToUpper(string(side)))
	params.Set("type", "LIMIT")
	params.Set("timeInForce", "GTC")
	params.Set("price", price.String())
	params.Set("quantity", quantity.String())
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", "5000") // 5 seconds - Binance recommended value

	// Check if we have credentials
	if bc.apiKey == "" || bc.apiSecret == "" {
		return nil, fmt.Errorf("Binance API credentials not configured - cannot place orders")
	}

	// Add signature
	signature := bc.sign(params.Encode())
	params.Set("signature", signature)

	req, err := http.NewRequest("POST", bc.baseURL+"/api/v3/order", strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-MBX-APIKEY", bc.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := bc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Log rate limit headers for monitoring
	if weight := resp.Header.Get("X-MBX-USED-WEIGHT-1M"); weight != "" {
		log.Printf("Binance API weight used: %s/6000", weight)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.Unmarshal(body, &errResp)

		// Special handling for rate limit errors
		if resp.StatusCode == 429 {
			retryAfter := resp.Header.Get("Retry-After")
			return nil, fmt.Errorf("binance rate limit exceeded (429), retry after: %s, error: %v", retryAfter, errResp)
		}

		return nil, fmt.Errorf("binance error %d: %v", resp.StatusCode, errResp)
	}

	var order models.BinanceOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	// Store in cache
	bc.storeInCache(cacheKey, &order)

	return &order, nil
}

// GetOrder retrieves order status from Binance
func (bc *BinanceClient) GetOrder(symbol, orderID string) (*models.BinanceOrder, error) {
	// Check if we have credentials
	if bc.apiKey == "" || bc.apiSecret == "" {
		return nil, fmt.Errorf("Binance API credentials not configured - cannot get order status")
	}

	params := url.Values{}
	params.Set("symbol", bc.formatSymbol(symbol))
	params.Set("orderId", orderID)
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", "5000")

	signature := bc.sign(params.Encode())
	params.Set("signature", signature)

	req, err := http.NewRequest("GET", bc.baseURL+"/api/v3/order?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-MBX-APIKEY", bc.apiKey)

	resp, err := bc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.Unmarshal(body, &errResp)
		return nil, fmt.Errorf("binance error %d: %v", resp.StatusCode, errResp)
	}

	var order models.BinanceOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	return &order, nil
}

// GetOpenOrders retrieves all open orders for a symbol
func (bc *BinanceClient) GetOpenOrders(symbol string) ([]*models.BinanceOrder, error) {
	// Check if we have credentials
	if bc.apiKey == "" || bc.apiSecret == "" {
		return nil, fmt.Errorf("Binance API credentials not configured - cannot get open orders")
	}

	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", bc.formatSymbol(symbol))
	}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("recvWindow", "5000")

	signature := bc.sign(params.Encode())
	params.Set("signature", signature)

	req, err := http.NewRequest("GET", bc.baseURL+"/api/v3/openOrders?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-MBX-APIKEY", bc.apiKey)

	resp, err := bc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		json.Unmarshal(body, &errResp)
		return nil, fmt.Errorf("binance error %d: %v", resp.StatusCode, errResp)
	}

	var orders []*models.BinanceOrder
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, err
	}

	// Update cache
	bc.updateCache(orders)

	return orders, nil
}

// Helper functions

func (bc *BinanceClient) sign(payload string) string {
	h := hmac.New(sha256.New, []byte(bc.apiSecret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

func (bc *BinanceClient) formatSymbol(symbol string) string {
	// Convert ETH to ETHUSDT, BTC to BTCUSDT, etc.
	if !strings.HasSuffix(symbol, "USDT") {
		return symbol + "USDT"
	}
	return symbol
}

// Cache management for idempotency

func (bc *BinanceClient) createCacheKey(symbol string, side models.OrderSide, price, quantity decimal.Decimal) string {
	// Use 0.01% tolerance for quantity
	tolerance := quantity.Mul(decimal.NewFromFloat(0.0001))
	roundedQty := quantity.Div(tolerance).Round(0).Mul(tolerance)

	return fmt.Sprintf("%s_%s_%s_%s",
		bc.formatSymbol(symbol),
		side,
		price.String(),
		roundedQty.String(),
	)
}

func (bc *BinanceClient) getFromCache(key string) *models.BinanceOrder {
	bc.orderCacheMutex.RLock()
	defer bc.orderCacheMutex.RUnlock()

	if time.Since(bc.lastCacheUpdate) > bc.cacheExpiry {
		return nil
	}

	return bc.orderCache[key]
}

func (bc *BinanceClient) storeInCache(key string, order *models.BinanceOrder) {
	bc.orderCacheMutex.Lock()
	defer bc.orderCacheMutex.Unlock()

	bc.orderCache[key] = order
	bc.lastCacheUpdate = time.Now()
}

func (bc *BinanceClient) updateCache(orders []*models.BinanceOrder) {
	bc.orderCacheMutex.Lock()
	defer bc.orderCacheMutex.Unlock()

	// Clear old cache
	bc.orderCache = make(map[string]*models.BinanceOrder)

	// Rebuild cache with current orders
	for _, order := range orders {
		if order.Status == "NEW" || order.Status == "PARTIALLY_FILLED" {
			price, _ := decimal.NewFromString(order.Price)
			qty, _ := decimal.NewFromString(order.OrigQty)
			side := models.SideBuy
			if strings.ToLower(order.Side) == "sell" {
				side = models.SideSell
			}

			key := bc.createCacheKey(order.Symbol, side, price, qty)
			bc.orderCache[key] = order
		}
	}

	bc.lastCacheUpdate = time.Now()
}

// ConvertBinanceStatus converts Binance order status to our format
func ConvertBinanceStatus(status string) string {
	switch status {
	case "NEW", "PARTIALLY_FILLED":
		return "open"
	case "FILLED":
		return "filled"
	case "CANCELED", "REJECTED", "EXPIRED":
		return "cancelled"
	default:
		return "open"
	}
}

// getSymbolInfo fetches and caches symbol trading rules
func (bc *BinanceClient) getSymbolInfo(symbol string) (*SymbolInfo, error) {
	bc.symbolInfoMutex.RLock()
	if info, ok := bc.symbolInfo[symbol]; ok && time.Since(bc.symbolInfoTime) < 24*time.Hour {
		bc.symbolInfoMutex.RUnlock()
		return info, nil
	}
	bc.symbolInfoMutex.RUnlock()

	// Fetch exchange info
	req, err := http.NewRequest("GET", bc.baseURL+"/api/v3/exchangeInfo?symbol="+symbol, nil)
	if err != nil {
		return nil, err
	}

	resp, err := bc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get exchange info: %s", body)
	}

	var exchangeInfo struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Filters []struct {
				FilterType  string `json:"filterType"`
				MinQty      string `json:"minQty,omitempty"`
				MaxQty      string `json:"maxQty,omitempty"`
				StepSize    string `json:"stepSize,omitempty"`
				MinPrice    string `json:"minPrice,omitempty"`
				MaxPrice    string `json:"maxPrice,omitempty"`
				TickSize    string `json:"tickSize,omitempty"`
				MinNotional string `json:"minNotional,omitempty"`
			} `json:"filters"`
		} `json:"symbols"`
	}

	if err := json.Unmarshal(body, &exchangeInfo); err != nil {
		return nil, err
	}

	if len(exchangeInfo.Symbols) == 0 {
		return nil, fmt.Errorf("symbol %s not found", symbol)
	}

	info := &SymbolInfo{
		MinQty:      decimal.NewFromFloat(0.00001),
		MaxQty:      decimal.NewFromFloat(10000000),
		StepSize:    decimal.NewFromFloat(0.00001),
		MinPrice:    decimal.NewFromFloat(0.01),
		MaxPrice:    decimal.NewFromFloat(1000000),
		TickSize:    decimal.NewFromFloat(0.01),
		MinNotional: decimal.NewFromFloat(10),
	}

	// Parse filters
	for _, filter := range exchangeInfo.Symbols[0].Filters {
		switch filter.FilterType {
		case "LOT_SIZE":
			if v, err := decimal.NewFromString(filter.MinQty); err == nil {
				info.MinQty = v
			}
			if v, err := decimal.NewFromString(filter.MaxQty); err == nil {
				info.MaxQty = v
			}
			if v, err := decimal.NewFromString(filter.StepSize); err == nil {
				info.StepSize = v
			}
		case "PRICE_FILTER":
			if v, err := decimal.NewFromString(filter.MinPrice); err == nil {
				info.MinPrice = v
			}
			if v, err := decimal.NewFromString(filter.MaxPrice); err == nil {
				info.MaxPrice = v
			}
			if v, err := decimal.NewFromString(filter.TickSize); err == nil {
				info.TickSize = v
			}
		case "MIN_NOTIONAL":
			if v, err := decimal.NewFromString(filter.MinNotional); err == nil {
				info.MinNotional = v
			}
		}
	}

	// Cache the info
	bc.symbolInfoMutex.Lock()
	bc.symbolInfo[symbol] = info
	bc.symbolInfoTime = time.Now()
	bc.symbolInfoMutex.Unlock()

	return info, nil
}

// roundToStepSize rounds a quantity to the nearest valid step size
func (bc *BinanceClient) roundToStepSize(quantity, stepSize decimal.Decimal) decimal.Decimal {
	if stepSize.IsZero() {
		return quantity
	}
	return quantity.Div(stepSize).Round(0).Mul(stepSize)
}

// roundUpToStepSize rounds quantity UP to the nearest step size
func (bc *BinanceClient) roundUpToStepSize(quantity, stepSize decimal.Decimal) decimal.Decimal {
	if stepSize.IsZero() {
		return quantity
	}
	return quantity.Div(stepSize).Ceil().Mul(stepSize)
}

// roundToTickSize rounds a price to the nearest valid tick size
func (bc *BinanceClient) roundToTickSize(price, tickSize decimal.Decimal) decimal.Decimal {
	if tickSize.IsZero() {
		return price
	}
	return price.Div(tickSize).Round(0).Mul(tickSize)
}
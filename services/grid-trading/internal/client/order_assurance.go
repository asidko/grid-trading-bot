package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

type OrderRequest struct {
	Symbol string          `json:"symbol"`
	Price  decimal.Decimal `json:"price"`
	Side   OrderSide       `json:"side"`
	Amount decimal.Decimal `json:"amount"`
}

type OrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type OrderStatus struct {
	OrderID      string           `json:"order_id"`
	Status       string           `json:"status"`
	FilledAmount *decimal.Decimal `json:"filled_amount,omitempty"`
	FillPrice    *decimal.Decimal `json:"fill_price,omitempty"`
}

type OrderAssuranceClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewOrderAssuranceClient(baseURL string) *OrderAssuranceClient {
	return &OrderAssuranceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *OrderAssuranceClient) PlaceOrder(req OrderRequest) (*OrderResponse, error) {
	url := fmt.Sprintf("%s/order-assurance", c.baseURL)

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error response
		var errorResp map[string]string
		if err := json.Unmarshal(body, &errorResp); err == nil {
			if msg, ok := errorResp["message"]; ok {
				return nil, fmt.Errorf("%s", msg)
			}
		}
		return nil, fmt.Errorf("unexpected status code: %d - %s", resp.StatusCode, string(body))
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &orderResp, nil
}

func (c *OrderAssuranceClient) GetOrderStatus(orderID string) (*OrderStatus, error) {
	url := fmt.Sprintf("%s/order-status/%s", c.baseURL, orderID)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var status OrderStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}
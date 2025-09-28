package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

type GridTradingClient struct {
	baseURL    string
	httpClient *http.Client
}

type PriceTrigger struct {
	Symbol string          `json:"symbol"`
	Price  decimal.Decimal `json:"price"`
}

func NewGridTradingClient(baseURL string) *GridTradingClient {
	return &GridTradingClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *GridTradingClient) SendPriceTrigger(symbol string, price decimal.Decimal) error {
	trigger := PriceTrigger{
		Symbol: symbol,
		Price:  price,
	}

	data, err := json.Marshal(trigger)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/trigger-for-price",
		"application/json",
		bytes.NewBuffer(data),
	)
	if err != nil {
		return fmt.Errorf("failed to send trigger: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
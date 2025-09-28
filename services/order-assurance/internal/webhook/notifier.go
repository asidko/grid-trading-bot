package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/grid-trading-bot/services/order-assurance/internal/models"
)

type Notifier struct {
	gridTradingURL string
	client         *http.Client
	maxRetries     int
	retryDelay     time.Duration
}

func NewNotifier(gridTradingURL string) *Notifier {
	return &Notifier{
		gridTradingURL: gridTradingURL,
		client:         &http.Client{Timeout: 10 * time.Second},
		maxRetries:     3,
		retryDelay:     1 * time.Second,
	}
}

// SendFillNotification sends fill notification to grid-trading service
func (n *Notifier) SendFillNotification(notification models.FillNotification) error {
	url := fmt.Sprintf("%s/order-fill-notification", n.gridTradingURL)

	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	for attempt := 1; attempt <= n.maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := n.client.Do(req)
		if err != nil {
			if attempt < n.maxRetries {
				log.Printf("Failed to send fill notification (attempt %d/%d): %v", attempt, n.maxRetries, err)
				time.Sleep(n.retryDelay * time.Duration(attempt))
				continue
			}
			return fmt.Errorf("failed to send notification after %d attempts: %w", n.maxRetries, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Successfully sent fill notification for order %s", notification.OrderID)
			return nil
		}

		if attempt < n.maxRetries {
			log.Printf("Received status %d for fill notification (attempt %d/%d)", resp.StatusCode, attempt, n.maxRetries)
			time.Sleep(n.retryDelay * time.Duration(attempt))
			continue
		}

		return fmt.Errorf("failed with status %d after %d attempts", resp.StatusCode, n.maxRetries)
	}

	return nil
}

// SendErrorNotification sends error notification to grid-trading service
func (n *Notifier) SendErrorNotification(notification models.ErrorNotification) error {
	url := fmt.Sprintf("%s/order-fill-error-notification", n.gridTradingURL)

	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	for attempt := 1; attempt <= n.maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := n.client.Do(req)
		if err != nil {
			if attempt < n.maxRetries {
				log.Printf("Failed to send error notification (attempt %d/%d): %v", attempt, n.maxRetries, err)
				time.Sleep(n.retryDelay * time.Duration(attempt))
				continue
			}
			return fmt.Errorf("failed to send notification after %d attempts: %w", n.maxRetries, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Successfully sent error notification for order %s", notification.OrderID)
			return nil
		}

		if attempt < n.maxRetries {
			log.Printf("Received status %d for error notification (attempt %d/%d)", resp.StatusCode, attempt, n.maxRetries)
			time.Sleep(n.retryDelay * time.Duration(attempt))
			continue
		}

		return fmt.Errorf("failed with status %d after %d attempts", resp.StatusCode, n.maxRetries)
	}

	return nil
}
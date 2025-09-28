package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/grid-trading-bot/services/grid-trading/internal/service"
	"github.com/shopspring/decimal"
)

type Handlers struct {
	gridService *service.GridService
}

func NewHandlers(gridService *service.GridService) *Handlers {
	return &Handlers{
		gridService: gridService,
	}
}

func (h *Handlers) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/trigger-for-price", h.handlePriceTrigger).Methods("POST")
	r.HandleFunc("/order-fill-notification", h.handleFillNotification).Methods("POST")
	r.HandleFunc("/order-fill-error-notification", h.handleErrorNotification).Methods("POST")
	r.HandleFunc("/health", h.handleHealth).Methods("GET")
}

type PriceTriggerRequest struct {
	Symbol string          `json:"symbol"`
	Price  decimal.Decimal `json:"price"`
}

type FillNotificationRequest struct {
	OrderID      string          `json:"order_id"`
	Symbol       string          `json:"symbol"`
	Price        decimal.Decimal `json:"price"`
	Side         string          `json:"side"`
	Status       string          `json:"status"`
	FilledAmount decimal.Decimal `json:"filled_amount"`
	FillPrice    decimal.Decimal `json:"fill_price"`
}

type ErrorNotificationRequest struct {
	OrderID string `json:"order_id"`
	Symbol  string `json:"symbol"`
	Side    string `json:"side"`
	Error   string `json:"error"`
}

func (h *Handlers) handlePriceTrigger(w http.ResponseWriter, r *http.Request) {
	var req PriceTriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Received price trigger for %s at %s", req.Symbol, req.Price)

	if err := h.gridService.ProcessPriceTrigger(req.Symbol, req.Price); err != nil {
		log.Printf("Error processing price trigger: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "processed"})
}

func (h *Handlers) handleFillNotification(w http.ResponseWriter, r *http.Request) {
	var req FillNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Status != "filled" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ignored"})
		return
	}

	log.Printf("Received fill notification for order %s (%s %s at %s)",
		req.OrderID, req.Side, req.Symbol, req.Price)

	var err error
	if req.Side == "buy" {
		err = h.gridService.ProcessBuyFillNotification(req.OrderID, req.FilledAmount)
	} else if req.Side == "sell" {
		err = h.gridService.ProcessSellFillNotification(req.OrderID)
	} else {
		http.Error(w, "Invalid side", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("Error processing fill notification: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "processed"})
}

func (h *Handlers) handleErrorNotification(w http.ResponseWriter, r *http.Request) {
	var req ErrorNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Received error notification for order %s: %s", req.OrderID, req.Error)

	if err := h.gridService.ProcessErrorNotification(req.OrderID, req.Side, req.Error); err != nil {
		log.Printf("Error processing error notification: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "processed"})
}

func (h *Handlers) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}
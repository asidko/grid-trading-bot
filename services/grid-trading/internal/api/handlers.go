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
	// Grid management endpoints
	r.HandleFunc("/grids", h.handleCreateGrid).Methods("POST")
	r.HandleFunc("/grids", h.handleGetAllGrids).Methods("GET")
	r.HandleFunc("/grids/{symbol}", h.handleGetGrids).Methods("GET")

	// Webhook endpoints
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

type CreateGridRequest struct {
	Symbol   string          `json:"symbol"`
	MinPrice decimal.Decimal `json:"min_price"`
	MaxPrice decimal.Decimal `json:"max_price"`
	GridStep decimal.Decimal `json:"grid_step"`
	BuyAmount decimal.Decimal `json:"buy_amount"`
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
		err = h.gridService.ProcessBuyFillNotification(req.OrderID, req.FilledAmount, req.FillPrice)
	} else if req.Side == "sell" {
		err = h.gridService.ProcessSellFillNotification(req.OrderID, req.FilledAmount, req.FillPrice)
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
	// Check database connectivity
	if err := h.gridService.CheckHealth(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (h *Handlers) handleCreateGrid(w http.ResponseWriter, r *http.Request) {
	var req CreateGridRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Symbol == "" {
		http.Error(w, "Symbol is required", http.StatusBadRequest)
		return
	}
	if req.MinPrice.LessThanOrEqual(decimal.Zero) || req.MaxPrice.LessThanOrEqual(decimal.Zero) {
		http.Error(w, "Min and max prices must be positive", http.StatusBadRequest)
		return
	}
	if req.MinPrice.GreaterThanOrEqual(req.MaxPrice) {
		http.Error(w, "Min price must be less than max price", http.StatusBadRequest)
		return
	}
	if req.GridStep.LessThanOrEqual(decimal.Zero) {
		http.Error(w, "Grid step must be positive", http.StatusBadRequest)
		return
	}
	if req.BuyAmount.LessThanOrEqual(decimal.Zero) {
		http.Error(w, "Buy amount must be positive", http.StatusBadRequest)
		return
	}

	log.Printf("Creating grid for %s: min=%s, max=%s, step=%s, amount=%s",
		req.Symbol, req.MinPrice, req.MaxPrice, req.GridStep, req.BuyAmount)

	_, err := h.gridService.CreateGrid(req.Symbol, req.MinPrice, req.MaxPrice, req.GridStep, req.BuyAmount)
	if err != nil {
		log.Printf("Error creating grid: %v", err)
		http.Error(w, "Failed to create grid", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) handleGetGrids(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	log.Printf("Fetching grid levels for symbol: %s", symbol)

	levels, err := h.gridService.GetGridLevels(symbol)
	if err != nil {
		log.Printf("Error fetching grid levels: %v", err)
		http.Error(w, "Failed to fetch grid levels", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(levels)
}

func (h *Handlers) handleGetAllGrids(w http.ResponseWriter, r *http.Request) {
	log.Printf("Fetching all grid levels")

	levels, err := h.gridService.GetAllGridLevels()
	if err != nil {
		log.Printf("Error fetching all grid levels: %v", err)
		http.Error(w, "Failed to fetch grid levels", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(levels)
}
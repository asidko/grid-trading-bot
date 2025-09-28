package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/grid-trading-bot/services/order-assurance/internal/models"
	"github.com/grid-trading-bot/services/order-assurance/internal/service"
)

type Handlers struct {
	orderService *service.OrderService
}

func NewHandlers(orderService *service.OrderService) *Handlers {
	return &Handlers{
		orderService: orderService,
	}
}

func (h *Handlers) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/order-assurance", h.handlePlaceOrder).Methods("POST")
	r.HandleFunc("/order-status/{order_id}", h.handleGetOrderStatus).Methods("GET")
	r.HandleFunc("/health", h.handleHealth).Methods("GET")
}

// handlePlaceOrder handles idempotent order placement
func (h *Handlers) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	var req models.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Received order request: %s %s at %s, amount: %s",
		req.Side, req.Symbol, req.Price, req.Amount)

	// Validate request
	if req.Symbol == "" || req.Price.IsZero() || req.Amount.IsZero() {
		http.Error(w, "Invalid order parameters", http.StatusBadRequest)
		return
	}

	// Place order (idempotent)
	resp, err := h.orderService.PlaceOrder(req)
	if err != nil {
		log.Printf("Error placing order: %v", err)

		// Parse Binance error codes
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "insufficient") || strings.Contains(errorMsg, "balance") {
			errorResp := map[string]string{
				"error": "insufficient_funds",
				"message": errorMsg,
			}
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorResp)
			return
		} else if strings.Contains(errorMsg, "MIN_NOTIONAL") {
			errorResp := map[string]string{
				"error": "order_too_small",
				"message": "Order value below minimum",
			}
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(errorResp)
			return
		}

		http.Error(w, "Failed to place order", http.StatusInternalServerError)
		return
	}

	log.Printf("Order placed successfully: %s", resp.OrderID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleGetOrderStatus retrieves order status from Binance
func (h *Handlers) handleGetOrderStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["order_id"]

	if orderID == "" {
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}

	status, err := h.orderService.GetOrderStatus(orderID)
	if err != nil {
		log.Printf("Error getting order status: %v", err)
		http.Error(w, "Failed to get order status", http.StatusInternalServerError)
		return
	}

	if status == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Order not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleHealth returns service health status
func (h *Handlers) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}
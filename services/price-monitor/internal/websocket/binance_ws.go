package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
)

const (
	BinanceWSURL = "wss://stream.binance.com:9443/ws"
	PingInterval = 30 * time.Second
	ReadTimeout  = 60 * time.Second
	WriteTimeout = 10 * time.Second
)

type PriceUpdate struct {
	Symbol string
	Price  decimal.Decimal
}

type BinanceWS struct {
	url     string
	symbols []string

	conn      *websocket.Conn
	connMutex sync.Mutex

	priceChan chan PriceUpdate
	errorChan chan error

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewBinanceWS(symbols []string) *BinanceWS {
	ctx, cancel := context.WithCancel(context.Background())

	return &BinanceWS{
		url:       BinanceWSURL,
		symbols:   symbols,
		priceChan: make(chan PriceUpdate, 1000), // Larger buffer to prevent blocking
		errorChan: make(chan error, 10),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (ws *BinanceWS) Connect() error {
	// Build stream URL with all symbols
	var streams []string
	for _, symbol := range ws.symbols {
		// Convert ETH to ethusdt@trade format
		streamName := strings.ToLower(symbol) + "usdt@trade"
		streams = append(streams, streamName)
	}

	streamURL := fmt.Sprintf("%s/%s", ws.url, strings.Join(streams, "/"))

	conn, _, err := websocket.DefaultDialer.Dial(streamURL, nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	ws.connMutex.Lock()
	ws.conn = conn
	ws.connMutex.Unlock()

	// Start goroutines
	ws.wg.Add(2)
	go ws.readLoop()
	go ws.pingLoop()

	return nil
}

func (ws *BinanceWS) readLoop() {
	defer ws.wg.Done()
	defer ws.closeConnection()

	for {
		select {
		case <-ws.ctx.Done():
			return
		default:
		}

		ws.connMutex.Lock()
		conn := ws.conn
		ws.connMutex.Unlock()

		if conn == nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(ReadTimeout))
			return nil
		})

		_, message, err := conn.ReadMessage()
		if err != nil {
			select {
			case ws.errorChan <- fmt.Errorf("read error: %w", err):
			case <-ws.ctx.Done():
			}
			return
		}

		// Parse trade message
		var trade struct {
			EventType string `json:"e"`
			Symbol    string `json:"s"`
			Price     string `json:"p"`
		}

		if err := json.Unmarshal(message, &trade); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		if trade.EventType != "trade" {
			continue
		}

		price, err := decimal.NewFromString(trade.Price)
		if err != nil {
			log.Printf("Invalid price format: %s", trade.Price)
			continue
		}

		// Remove USDT suffix
		symbol := strings.TrimSuffix(trade.Symbol, "USDT")

		select {
		case ws.priceChan <- PriceUpdate{
			Symbol: symbol,
			Price:  price,
		}:
		case <-ws.ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
			// Log if channel is consistently full
			log.Printf("Price channel full, dropping update for %s", symbol)
		}
	}
}

func (ws *BinanceWS) pingLoop() {
	defer ws.wg.Done()

	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ws.ctx.Done():
			return

		case <-ticker.C:
			ws.connMutex.Lock()
			conn := ws.conn
			ws.connMutex.Unlock()

			if conn == nil {
				return
			}

			deadline := time.Now().Add(WriteTimeout)
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, deadline); err != nil {
				select {
				case ws.errorChan <- fmt.Errorf("ping failed: %w", err):
				case <-ws.ctx.Done():
				}
				return
			}
		}
	}
}

func (ws *BinanceWS) closeConnection() {
	ws.connMutex.Lock()
	defer ws.connMutex.Unlock()

	if ws.conn != nil {
		ws.conn.Close()
		ws.conn = nil
	}
}

func (ws *BinanceWS) PriceChannel() <-chan PriceUpdate {
	return ws.priceChan
}

func (ws *BinanceWS) ErrorChannel() <-chan error {
	return ws.errorChan
}

func (ws *BinanceWS) Close() error {
	// Signal shutdown
	ws.cancel()

	// Close connection
	ws.closeConnection()

	// Wait for goroutines
	ws.wg.Wait()

	// Close channels
	close(ws.priceChan)
	close(ws.errorChan)

	return nil
}
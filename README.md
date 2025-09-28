# Grid Trading Bot

A multi-service Golang application implementing a grid trading strategy for cryptocurrency markets using Binance Spot API.

## Architecture

The system consists of two microservices:

### 1. Grid Trading Service
Manages grid levels and trading logic:
- Places buy orders when price drops to predefined levels
- Places sell orders when price rises after a buy fill
- Operates independently across multiple price levels
- Handles crash recovery and missed notifications

### 2. Order Assurance Service
Interfaces with Binance Spot API:
- Places idempotent limit orders on Binance
- Monitors orders for fills
- Sends webhook notifications back to grid-trading service
- Uses Binance as the source of truth (no local database)

## Quick Start

1. **Setup Database**
   ```bash
   make docker-up
   ```

2. **Configure Environment**
   ```bash
   cp .env.example .env
   # Edit .env with your Binance API credentials
   ```

3. **Build Services**
   ```bash
   make build-all
   ```

4. **Run Services**
   ```bash
   # Run both services
   make run-all

   # Or run individually:
   make run-assurance  # Start order-assurance first
   make run-grid       # Then start grid-trading
   ```

5. **Initialize Grid Levels**
   ```bash
   # Example: ETH grid from $2000-$4000 with $200 steps
   make init-grid SYMBOL=ETH MIN=2000 MAX=4000 STEP=200 AMOUNT=1000
   ```

## API Endpoints

- `POST /trigger-for-price` - Process price updates
- `POST /order-fill-notification` - Handle order fills
- `POST /order-fill-error-notification` - Handle order errors
- `GET /health` - Health check

## State Machine

```
READY → PLACING_BUY → BUY_ACTIVE → HOLDING → PLACING_SELL → SELL_ACTIVE → READY
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | HTTP server port | 8080 |
| `DB_HOST` | PostgreSQL host | localhost |
| `DB_PORT` | PostgreSQL port | 5432 |
| `ORDER_ASSURANCE_URL` | Order service URL | http://localhost:9090 |
| `SYNC_JOB_ENABLED` | Enable recovery job | true |
| `SYNC_JOB_CRON` | Sync schedule | 0 * * * * (hourly) |

## Development

```bash
# Run tests
make test

# Clean build artifacts
make clean

# Stop database
make docker-down
```
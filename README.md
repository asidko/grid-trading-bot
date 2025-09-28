# Grid Trading Bot

A multi-service Golang application implementing a grid trading strategy for cryptocurrency markets using Binance Spot API.

## Architecture

The system consists of three microservices:

### 1. Grid Trading Service (Port 8080)
Manages grid levels and trading logic:
- Places buy orders when price drops to predefined levels
- Places sell orders when price rises after a buy fill
- Operates independently across multiple price levels
- Handles crash recovery and missed notifications

### 2. Order Assurance Service (Port 9090)
Interfaces with Binance Spot API:
- Places idempotent limit orders on Binance
- Enforces symbol trading restrictions (min quantity, tick size)
- Sends webhook notifications back to grid-trading service
- Uses Binance as the source of truth (no local database)

### 3. Price Monitor Service (Port 7070)
Real-time price monitoring:
- Connects to Binance WebSocket for live price feeds
- Triggers grid-trading service on price changes
- Auto-reconnects with exponential backoff
- Configurable price change thresholds

## Quick Start with Docker Compose

1. **Configure Environment**
   ```bash
   cp .env.example .env
   # Edit .env with your Binance API credentials
   ```

2. **Start All Services**
   ```bash
   docker-compose up -d
   ```

   > **Note:** Services use host network mode, so they'll be accessible on localhost:
   > - Grid Trading: http://localhost:8080
   > - Order Assurance: http://localhost:9090
   > - Price Monitor: http://localhost:7070
   > - PostgreSQL: localhost:5432

3. **Check Health**
   ```bash
   ./scripts/health-check.sh
   ```

4. **View Logs**
   ```bash
   docker-compose logs -f
   ```

5. **Initialize Grid Levels**
   ```bash
   # Example: ETH grid from $3000-$4000 with $100 steps
   make init-grid SYMBOL=ETH MIN=3000 MAX=4000 STEP=100 AMOUNT=1000
   ```

6. **Stop Services**
   ```bash
   docker-compose down
   ```

## Local Development

1. **Start PostgreSQL**
   ```bash
   docker-compose up -d postgres
   ```

2. **Configure Environment**
   ```bash
   cp .env.example .env
   # Edit .env - use DB_HOST=localhost for local dev
   ```

3. **Run Services**
   ```bash
   # Run all services locally
   make run-all

   # Or run individually:
   make run-assurance  # Order assurance service
   make run-grid       # Grid trading service
   make run-monitor    # Price monitor service
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
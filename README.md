# Grid Trading Bot

A multi-service Golang application implementing a grid trading strategy for cryptocurrency markets.

## Architecture

The bot implements a reactive grid trading strategy that:
- Places buy orders when price drops to predefined levels
- Places sell orders when price rises after a buy fill
- Operates independently across multiple price levels
- Handles crash recovery and missed notifications

## Quick Start

1. **Setup Database**
   ```bash
   make docker-up
   ```

2. **Configure Environment**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

3. **Build and Run**
   ```bash
   make build
   make run
   ```

4. **Initialize Grid Levels**
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
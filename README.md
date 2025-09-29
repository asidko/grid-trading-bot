# Grid Trading Bot

Automated cryptocurrency trading bot that profits from market volatility by buying low and selling high at predefined price levels.

## What is Grid Trading?

Grid trading splits a price range into levels. The bot automatically buys when price drops to each level, then sells when price rises.

**Example**: ETH grid from $3000-$4000 with $100 steps
- Buy at $3000 → Sell at $3100 → $100 profit
- Buy at $3100 → Sell at $3200 → $100 profit
- And so on...

Each level operates independently, generating profits from price movements in both directions.

## Documentation

- **[IDEA.md](IDEA.md)** - Strategy concept
- **[SPEC.md](SPEC.md)** - Technical specification

## Quick Start

### 1. Setup

```bash
# Copy and edit config
cp .env.example .env
nano .env  # Add your Binance API keys
```

**Required**:
- `BINANCE_API_KEY` - Your API key
- `BINANCE_API_SECRET` - Your API secret

**Optional**:
- `MONITORED_SYMBOLS=ETH,BTC` - Symbols to trade
- `PRICE_CHECK_INTERVAL_MS=10000` - Check prices every 10s
- `TRIGGER_INTERVAL_MS=5000` - Min 5s between triggers

### 2. Start

```bash
docker-compose up -d
```

Services run on localhost:
- Grid Trading: 8080
- Order Assurance: 9090
- Price Monitor: 7070

### 3. Initialize Grid

```bash
# Example: ETH between $3000-$4000, $100 steps, $1000 per trade
make init-grid SYMBOL=ETH MIN=3000 MAX=4000 STEP=100 AMOUNT=1000
```

### 4. Monitor

```bash
# View logs
docker-compose logs -f

# Check status
curl http://localhost:7070/status

# Stop
docker-compose down
```

## Configuration

### Essential Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `BINANCE_API_KEY` | Binance API key | *required* |
| `BINANCE_API_SECRET` | Binance API secret | *required* |
| `MONITORED_SYMBOLS` | Symbols to trade | ETH,BTC,BNB |
| `PRICE_CHECK_INTERVAL_MS` | Price check frequency | 10000 (10s) |
| `TRIGGER_INTERVAL_MS` | Min time between triggers | 5000 (5s) |
| `MIN_PRICE_CHANGE_PCT` | Min price change to trigger | 0.01 (0.01%) |

### Service Ports

| Variable | Port | Service |
|----------|------|---------|
| `GRID_PORT` | 8080 | Grid trading logic |
| `ASSURANCE_PORT` | 9090 | Order placement |
| `MONITOR_PORT` | 7070 | Price monitoring |

<details>
<summary>All Configuration Options</summary>

**Database**:
- `DB_HOST=localhost`
- `DB_PORT=5432`
- `DB_USER=postgres`
- `DB_PASSWORD=postgres`
- `DB_NAME=grid_trading`

**Internal URLs**:
- `ORDER_ASSURANCE_URL=http://localhost:9090`
- `GRID_TRADING_URL=http://localhost:8080`

**Recovery Job**:
- `SYNC_JOB_ENABLED=true` - Hourly order sync
- `SYNC_JOB_CRON=0 * * * *` - Cron schedule

</details>

## Troubleshooting

**Bot not placing orders?**
```bash
docker-compose logs -f grid
# Check: Grid levels initialized? Price in range?
```

**Binance connection errors?**
- Verify API keys in `.env`
- Check API permissions (Spot trading enabled)
- Ensure system time is synced

**Price monitor not working?**
```bash
curl http://localhost:7070/status
# Check: Symbols match? MIN_PRICE_CHANGE_PCT too high?
```

## Development

```bash
# Run locally without Docker
docker-compose up -d postgres
make run-all
```

## Requirements

- Docker & Docker Compose
- Binance account ([Get API keys](https://www.binance.com/en/my/settings/api-management))
- USDT funds in Binance Spot account (5k for example, for investing 1k$ per level to cover at least 5 levels of price move; expected daily revenue ~30$/day on ETH at 2025)
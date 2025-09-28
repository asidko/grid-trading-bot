# Grid Trading Bot API Documentation

## Grid Management Endpoints

### Create Grid
Creates new grid levels for a symbol. This is an idempotent operation - it will only create missing levels and skip existing ones.

**Endpoint:** `POST /grids`

**Request Body:**
```json
{
  "symbol": "ETHUSDT",
  "min_price": "2000",
  "max_price": "4000",
  "grid_step": "100",
  "buy_amount": "0.01"
}
```

**Response:** `200 OK` (no body)

**Example:**
```bash
curl -X POST http://localhost:8080/grids \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "ETHUSDT",
    "min_price": "3000",
    "max_price": "3500",
    "grid_step": "50",
    "buy_amount": "0.01"
  }'
```

### Get Grid Levels by Symbol
Retrieves all grid levels for a specific symbol.

**Endpoint:** `GET /grids/{symbol}`

**Response:** `200 OK` with JSON array of grid levels

**Example:**
```bash
curl http://localhost:8080/grids/ETHUSDT
```

### Get All Grid Levels
Retrieves all grid levels across all symbols.

**Endpoint:** `GET /grids`

**Response:** `200 OK` with JSON array of all grid levels

**Example:**
```bash
curl http://localhost:8080/grids
```

## Webhook Endpoints

### Price Trigger
Triggered when price changes to check if any grid levels can be activated.

**Endpoint:** `POST /trigger-for-price`

**Request Body:**
```json
{
  "symbol": "ETHUSDT",
  "price": "3050.00"
}
```

**Response:** `200 OK`
```json
{
  "status": "processed"
}
```

### Order Fill Notification
Notifies when an order has been filled.

**Endpoint:** `POST /order-fill-notification`

**Request Body:**
```json
{
  "order_id": "123456",
  "symbol": "ETHUSDT",
  "price": "3050.00",
  "side": "buy",
  "status": "filled",
  "filled_amount": "0.01",
  "fill_price": "3050.00"
}
```

**Response:** `200 OK`
```json
{
  "status": "processed"
}
```

### Order Error Notification
Notifies when an order encounters an error.

**Endpoint:** `POST /order-fill-error-notification`

**Request Body:**
```json
{
  "order_id": "123456",
  "symbol": "ETHUSDT",
  "side": "buy",
  "error": "Insufficient balance"
}
```

**Response:** `200 OK`
```json
{
  "status": "processed"
}
```

## Health Check

**Endpoint:** `GET /health`

**Response:** `200 OK`
```json
{
  "status": "healthy"
}
```
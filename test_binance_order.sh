#!/bin/bash

# Source environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Check if API credentials are set
if [ -z "$BINANCE_API_KEY" ] || [ -z "$BINANCE_API_SECRET" ]; then
    echo "Error: BINANCE_API_KEY or BINANCE_API_SECRET not set in .env"
    exit 1
fi

# Get parameters
SYMBOL="${1:-ETHUSDT}"
ORDER_ID="${2:-36325283176}"

# Generate timestamp and signature
TIMESTAMP=$(date +%s000)
QUERY="symbol=${SYMBOL}&orderId=${ORDER_ID}&timestamp=${TIMESTAMP}"
SIG=$(echo -n "${QUERY}" | openssl dgst -sha256 -hmac "${BINANCE_API_SECRET}" | awk '{print $2}')

# Make request
echo "Querying Binance for order ${ORDER_ID} on ${SYMBOL}..."
curl -s "https://api.binance.com/api/v3/allOrders?${QUERY}&signature=${SIG}" \
  -H "X-MBX-APIKEY: ${BINANCE_API_KEY}" | jq .

echo ""
echo "If you get an error, try:"
echo "  ./test_binance_order.sh ETHUSDT 36325283176"
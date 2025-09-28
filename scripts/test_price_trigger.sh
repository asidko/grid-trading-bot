#!/bin/bash

# Test script for sending price triggers to the grid-trading service

GRID_TRADING_URL=${GRID_TRADING_URL:-"http://localhost:8080"}
SYMBOL=${1:-"ETH"}
PRICE=${2:-"3700"}

echo "Sending price trigger for $SYMBOL at $PRICE to $GRID_TRADING_URL"

curl -X POST "$GRID_TRADING_URL/trigger-for-price" \
  -H "Content-Type: application/json" \
  -d "{\"symbol\": \"$SYMBOL\", \"price\": $PRICE}" \
  -w "\nHTTP Status: %{http_code}\n"

echo ""
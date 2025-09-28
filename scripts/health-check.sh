#!/bin/bash

# Health check script for all services

echo "🔍 Checking Grid Trading Bot Services..."
echo "========================================"

# Check Grid Trading Service
echo -n "Grid Trading (port 8080): "
if curl -s -f http://localhost:8080/health > /dev/null 2>&1; then
    echo "✅ Healthy"
else
    echo "❌ Not responding"
fi

# Check Order Assurance Service
echo -n "Order Assurance (port 9090): "
if curl -s -f http://localhost:9090/health > /dev/null 2>&1; then
    echo "✅ Healthy"
else
    echo "❌ Not responding"
fi

# Check Price Monitor Service
echo -n "Price Monitor (port 7070): "
if curl -s -f http://localhost:7070/health > /dev/null 2>&1; then
    echo "✅ Healthy"
else
    echo "❌ Not responding"
fi

# Check PostgreSQL
echo -n "PostgreSQL (port 5432): "
if pg_isready -h localhost -p 5432 > /dev/null 2>&1; then
    echo "✅ Ready"
else
    echo "❌ Not ready"
fi

echo "========================================"

# Check Price Monitor Status
echo ""
echo "📊 Price Monitor Status:"
curl -s http://localhost:7070/status 2>/dev/null | jq '.' 2>/dev/null || echo "Unable to get status"
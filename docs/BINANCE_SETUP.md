# Binance API Setup Guide

## Getting Binance API Credentials

### Production API (Real Trading)

1. **Create/Login to Binance Account**
   - Go to [Binance.com](https://www.binance.com)
   - Complete KYC verification if required

2. **Generate API Keys**
   - Go to Account → API Management
   - Create a new API key
   - Label it (e.g., "Grid Trading Bot")
   - Complete 2FA verification

3. **Configure API Restrictions**
   - Enable **Spot Trading** permission
   - Restrict access to your IP address (recommended)
   - Save your API Key and Secret securely

### Testnet API (Testing)

1. **Access Binance Testnet**
   - Go to [testnet.binance.vision](https://testnet.binance.vision)
   - Login with GitHub account

2. **Generate Testnet API Keys**
   - Click "Generate HMAC_SHA256 Key"
   - Save the generated API Key and Secret

3. **Get Test Funds**
   - Use the faucet to get test USDT
   - Test funds are automatically replenished

## Configuration

Add your credentials to `.env`:

```env
# For Production
BINANCE_API_KEY=your_production_api_key
BINANCE_API_SECRET=your_production_api_secret
BINANCE_TESTNET=false

# For Testing
BINANCE_API_KEY=your_testnet_api_key
BINANCE_API_SECRET=your_testnet_api_secret
BINANCE_TESTNET=true
```

## Security Best Practices

1. **Never commit API credentials**
   - Keep `.env` in `.gitignore`
   - Use environment variables in production

2. **IP Whitelist**
   - Restrict API access to your server's IP

3. **Limited Permissions**
   - Only enable Spot Trading
   - Don't enable withdrawal permissions

4. **Separate Keys**
   - Use different API keys for testing and production
   - Rotate keys periodically

## Testing the Connection

1. **Start the order-assurance service**:
   ```bash
   make run-assurance
   ```

2. **Check health endpoint**:
   ```bash
   curl http://localhost:9090/health
   ```

3. **Test order placement** (testnet recommended):
   ```bash
   curl -X POST http://localhost:9090/order-assurance \
     -H "Content-Type: application/json" \
     -d '{
       "symbol": "ETH",
       "price": 2000,
       "side": "buy",
       "amount": 100
     }'
   ```

## Troubleshooting

### Common Errors

1. **"Invalid API key"**
   - Check API key and secret are correct
   - Ensure no extra spaces in .env file

2. **"Insufficient balance"**
   - Check account balance
   - For testnet, use faucet to get funds

3. **"Invalid symbol"**
   - Use correct format: ETH, BTC (not ETHUSDT)
   - Service automatically appends USDT

4. **"Order would immediately match"**
   - Buy price too high or sell price too low
   - Adjust grid levels

## Rate Limits

Binance has rate limits:
- **Order placement**: 100 orders/10 seconds
- **Order status**: 6000 requests/minute

The order-assurance service handles these automatically through:
- Caching for idempotency
- Monitoring interval configuration
- Automatic retry with backoff
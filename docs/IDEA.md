## Grid Trading Bot Spec

### The Opportunity
The crypto market is wildly unpredictable - countless analysts try to forecast whether prices will rise or fall, yet even experts get it wrong. But there's one thing we know with absolute certainty: the price never stands still. It constantly dances up and down, creating endless waves of movement. While others struggle to predict direction, we can profit from the motion itself. This bot doesn't care if ETH goes to $10,000 or $1,000 - it simply harvests profits from every fluctuation along the way.

### The Idea
The bot splits the price range into fixed levels (e.g., every $200). Here's the simple strategy: buy when price drops to any level, sell when it rises to the next level above. Each level works independently - buy at $2400, sell at $2600, pocket the difference. By spreading trades across multiple levels, we turn market volatility into consistent profits, harvesting gains from every up-and-down movement.

### Grid Setup
- **Range**: ETH $2000 - $4000
- **Levels**: Every $200 → $2000, $2200, $2400, ..., $4000
- **Trade Size**: $1000 USDT per level

### Trading Rules
1. **Buy**: When price hits a level → buy $1000 worth (only if not already holding)
2. **Sell**: When price reaches next level up → sell position from level below
3. **Track**: Each level's status (empty/filled) and quantity held
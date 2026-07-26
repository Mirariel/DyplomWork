# TradeTracker Go — Exchange Adapters

Всі адаптери знаходяться в `internal/services/exchange/`.

---

## Інтерфейси

### Exchange (базовий)
```go
type Exchange interface {
    Name() string
    GetBalances(creds Credentials) ([]Balance, error)
    GetOpenPositions(creds Credentials) ([]Position, error)
    GetClosedTrades(creds Credentials, startMs, endMs int64) ([]ClosedTrade, error)
    GetPrices(symbols []string) (map[string]float64, error)
}
```

### SpotTrader (опціональний — Binance, OKX, Bybit)
```go
type SpotTrader interface {
    GetRecentTrades(creds Credentials, startMs, endMs int64) ([]SpotTrade, error)
}
// Перевірка: if st, ok := ex.(exchange.SpotTrader); ok { ... }
```

### Trader (опціональний — Binance, OKX, Bybit)
```go
type Trader interface {
    PlaceOrder(creds Credentials, req PlaceOrderRequest) (PlacedOrder, error)
    CancelOrder(creds Credentials, req CancelOrderRequest) error
    GetOrderStatus(creds Credentials, req CancelOrderRequest) (PlacedOrder, error)
}
```

### Credentials
```go
type Credentials struct {
    APIKey     string
    APISecret  string
    Passphrase string  // тільки OKX і KuCoin
}
```

### ClosedTrade
```go
type ClosedTrade struct {
    Symbol      string
    Side        string  // "LONG" | "SHORT"
    Quantity    float64 // кількість контрактів (OKX SWAP) або монет
    EntryPrice  float64
    ClosePrice  float64
    PnL         float64
    Leverage    int     // 0 = невідомо (cross-margin OKX завжди повертає 0)
    MarginMode  string  // "cross" | "isolated" (з біржового API)
    Fee         float64 // абсолютне значення USDT
    NotionalUsd float64 // повний розмір позиції в USD (ctVal вже врахований)
    OpenedAt    int64   // Unix timestamp ms, 0 = невідомо
    ClosedAt    int64   // Unix timestamp ms
}
```

**Важливо (OKX SWAP):** `Quantity` — це `closeTotalPos` (кількість контрактів), не базова монета.
`NotionalUsd` = `qty × ctVal × entryPrice` — вже правильне значення в USD. Використовувати саме його.

### Position (open)
```go
type Position struct {
    Symbol          string
    Side            string  // "LONG" | "SHORT"
    Quantity        float64
    EntryPrice      float64
    MarkPrice       float64
    PnL             float64
    Leverage        int
    MarginType      string  // "cross" | "isolated"
    NotionalEntryUsd float64 // trade_size = qty × ctVal × entryPrice
    LiqPrice        float64
}
```

### InitialMargin helper
```go
// exchange.InitialMargin — єдина точка обчислення початкової маржі.
// Використовується в processPositions, processHistory, WS broadcast.
func InitialMargin(notionalEntry float64, leverage int) float64 {
    if leverage <= 0 || notionalEntry <= 0 { return 0 }
    return notionalEntry / float64(leverage)
}
```
**ROI** = `PnL / InitialMargin(notional, leverage) × 100` — збігається з формулою OKX.

---

## Підтримувані біржі

| Біржа | Читання | Торгівля | SpotTrader | Підпис |
|---|---|---|---|---|
| Binance | Spot + Futures | так | так | HMAC-SHA256 query string |
| OKX | Spot + Futures + Earn | так | так | HMAC-SHA256 Base64 + passphrase |
| Bybit | UTA (spot + futures) | так | так | HMAC-SHA256 body string |
| Gate.io | Spot + USDT-M Futures | ні | ні | HMAC-SHA512 |
| Kraken | Spot | ні | ні | HMAC-SHA512 + Base64 decode secret |
| KuCoin | Spot | ні | ні | HMAC-SHA256 Base64 |

---

## Binance

**Файли:** `binance.go`, `binance_trade.go`

**Auth схема (GET):**
```
payload  = queryString + "&timestamp=" + ts
signature = HMAC-SHA256(apiSecret, payload)
```
Підпис додається як `?signature=...` в query string.

**Auth схема (POST):**
```go
params := url.Values{}
params.Set("symbol", "BTCUSDT")
params.Set("timestamp", ts)
params.Set("signature", hmacSHA256(secret, params.Encode()))
// POST body: application/x-www-form-urlencoded
```

**Ендпоінти:**
| Дія | URL |
|---|---|
| Spot баланси | `GET /api/v3/account` |
| Futures позиції | `GET /fapi/v2/positionRisk` |
| Закриті угоди | `GET /fapi/v1/userTrades` |
| Spot trades | `GET /api/v3/myTrades` |
| Ціни | `GET /api/v3/ticker/price` |
| Розмістити spot | `POST /api/v3/order` |
| Розмістити futures | `POST /fapi/v1/order` |
| Скасувати spot | `DELETE /api/v3/order` |
| Статус spot | `GET /api/v3/order` |

**Особливості:**
- Futures позиції: фільтруємо де `|positionAmt| > 0`
- `symbol`: `"BTC"` → `"BTCUSDT"` (функція `binanceSymbol`)
- `recvWindow=5000` для допуску розсинхрону часу

---

## OKX

**Файли:** `okx.go`, `okx_trade.go`

**Auth схема:**
```
timestamp  = "2025-01-15T10:00:00.000Z"  (ISO 8601 UTC)
prehash    = timestamp + "GET" + "/api/v5/account/balance" + ""
signature  = Base64(HMAC-SHA256(apiSecret, prehash))
```

**Headers:**
```
OK-ACCESS-KEY: <apiKey>
OK-ACCESS-SIGN: <signature>
OK-ACCESS-TIMESTAMP: <timestamp>
OK-ACCESS-PASSPHRASE: <rawPassphrase>
```

**Ендпоінти:**
| Дія | URL |
|---|---|
| Баланси (Trading) | `GET /api/v5/account/balance` |
| Баланси (Funding) | `GET /api/v5/asset/balances` |
| Earn (Savings) | `GET /api/v5/finance/savings/balance` |
| Earn (Staking) | `GET /api/v5/finance/fixed-income/staking-data` |
| Earn (ETH2) | `GET /api/v5/finance/ethstaking/balance` |
| Відкриті позиції | `GET /api/v5/account/positions` |
| **Закриті угоди** | `GET /api/v5/account/positions-history?limit=100` |
| Leverage info | `GET /api/v5/account/leverage-info?instId={}&mgnMode={}` |
| **Contract info** | `GET /api/v5/public/instruments?instType=SWAP&instId={}` |
| Spot trades | `GET /api/v5/trade/fills?instType=SPOT` |
| Ціни | `GET /api/v5/market/tickers?instType=SPOT` |
| PlaceOrder | `POST /api/v5/trade/order` |
| CancelOrder | `POST /api/v5/trade/cancel-order` |

**Особливості — КРИТИЧНО для positions-history:**

### ctVal (contract value)
`closeTotalPos` — це кількість **контрактів**, не монет. Кожен інструмент має `ctVal`:
- `ONDO-USDT-SWAP`: ctVal = 10 → 1 контракт = 10 ONDO
- `ZEC-USDT-SWAP`: ctVal = 0.01 → 1 контракт = 0.01 ZEC
- `BTC-USDT-SWAP`: ctVal = 0.01 → 1 контракт = 0.01 BTC

**Правильний розрахунок notional:**
```go
notionalUsd = qty_contracts × ctVal × entryPrice
```
Або використати поле `notionalUsd` з відповіді (якщо не 0).

**Fallback (коли `notionalUsd = 0`):**
```go
// Фетчимо ctVal з публічного API (без авторизації)
GET /api/v5/public/instruments?instType=SWAP&instId=ONDO-USDT-SWAP
// → data[0].ctVal = "10"
notionalUsd = math.Abs(qty) * ctVal * entryPrice
```

### Leverage для cross-margin
`positions-history` завжди повертає `lever: "0"` для cross-margin угод.

**Fallback:**
```go
// Фетчимо поточне налаштування плеча
GET /api/v5/account/leverage-info?instId=ONDO-USDT-SWAP&mgnMode=cross
// → data[0].lever = "10" (поточне, не на момент угоди)
```

Якщо leverage невідомий → зберігати `"0x"` (sentinel). У UI показувати `"Cross"`.

### Структура відповіді positions-history
| OKX поле | Наш тип | Примітка |
|---|---|---|
| `instId` | Symbol | нормалізується через `normalizeSymbol()` |
| `posSide` | Side | "long"/"short"/"net" → "LONG"/"SHORT" |
| `mgnMode` | MarginMode | "cross"/"isolated" |
| `lever` | Leverage | "0" для cross-margin → fallback |
| `openAvgPx` | EntryPrice | ціна входу |
| `closeAvgPx` | ClosePrice | ціна виходу |
| `closeTotalPos` | Quantity | **кількість контрактів**, не монет! |
| `notionalUsd` | NotionalUsd | USD value (якщо 0 → ctVal lookup) |
| `realizedPnl` | PnL | реалізований PnL |
| `fee` | Fee | від'ємне значення → беремо abs() |
| `cTime` | OpenedAt | Unix ms — час відкриття |
| `uTime` | ClosedAt | Unix ms — час закриття |

---

## Bybit

**Файли:** `bybit.go`, `bybit_trade.go`

**Auth схема (GET):**
```
queryString = "category=linear&limit=50"
payload     = timestamp + apiKey + recvWindow + queryString
signature   = HMAC-SHA256(apiSecret, payload)
```

**Auth схема (POST — відрізняється!):**
```
jsonBody    = '{"category":"spot","symbol":"BTCUSDT",...}'
payload     = timestamp + apiKey + recvWindow + jsonBody
signature   = HMAC-SHA256(apiSecret, payload)
```

**Headers:**
```
X-BAPI-API-KEY: <apiKey>
X-BAPI-TIMESTAMP: <ts>
X-BAPI-RECV-WINDOW: 5000
X-BAPI-SIGN: <signature>
Content-Type: application/json  (тільки для POST)
```

**Ендпоінти (V5 UTA):**
| Дія | URL |
|---|---|
| Баланси | `GET /v5/account/wallet-balance?accountType=UNIFIED` |
| Позиції | `GET /v5/position/list?category=linear` |
| Закриті угоди | `GET /v5/position/closed-pnl?category=linear` |
| Ціни | `GET /v5/market/tickers?category=spot` |
| PlaceOrder | `POST /v5/order/create` |
| CancelOrder | `POST /v5/order/cancel` |
| GetOrderStatus | `GET /v5/order/realtime` |

**Особливості:**
- Пагінація по закритих угодах через `cursor` (до 7 днів назад)
- `category`: `"spot"` або `"linear"` (USDT perpetual futures)
- `side`: `"Buy"` або `"Sell"` (капіталізація!)
- `orderType`: `"Market"` або `"Limit"` (капіталізація!)

---

## Gate.io

**Файл:** `gate.go`

**Auth схема:**
```
timestamp    = "1736934000"  (Unix секунди, рядок)
bodyHash     = SHA512Hex("")  (порожній body для GET)
signPayload  = "GET\n/api/v4/spot/accounts\n\n" + bodyHash + "\n" + timestamp
signature    = HMAC-SHA512(apiSecret, signPayload)
```

**Headers:**
```
KEY: <apiKey>
SIGN: <signature>
Timestamp: <timestamp>
```

**Ендпоінти:**
| Дія | URL |
|---|---|
| Spot баланси | `GET /api/v4/spot/accounts` |
| Futures позиції | `GET /api/v4/futures/usdt/positions` |
| Закриті позиції | `GET /api/v4/futures/usdt/position_close` |
| Ціни | `GET /api/v4/spot/tickers` |

**Особливості:**
- Час у закритих позиціях — Unix секунди → × 1000 для мілісекунд
- Gate не підтримує Trader і SpotTrader інтерфейси

---

## Kraken

**Файл:** `kraken.go`

**Auth схема (унікальна!):**
```
nonce      = strconv.FormatInt(nowMs(), 10)
postData   = "nonce=" + nonce + "&" + otherParams
prehash    = SHA256(nonce + postData)         (бінарно)
keyBytes   = Base64Decode(apiSecret)          // ← Kraken зберігає secret у base64!
signature  = Base64(HMAC-SHA512(keyBytes, path + prehash))
```

**Headers:**
```
API-Key: <apiKey>
API-Sign: <signature>
Content-Type: application/x-www-form-urlencoded
```

**Ендпоінти:**
| Дія | URL |
|---|---|
| Баланси | `POST /0/private/Balance` |
| Угоди | `POST /0/private/TradesHistory` |

**Особливості:**
- Kraken не має публічного API futures → `GetOpenPositions()` повертає `[]`
- `GetPrices()` повертає `{}` (Binance/OKX покривають ціни)
- Нормалізація активів: `XXBT` → `BTC`, `ZUSD` → `USD`, `XETH` → `ETH`
- Використовує `postForm` (не `postJSON`)
- Не підтримує Trader і SpotTrader інтерфейси

---

## KuCoin

**Файл:** `kucoin.go`

**Auth схема:**
```
timestamp  = strconv.FormatInt(nowMs(), 10)
prehash    = timestamp + "GET" + "/api/v1/fills" + ""
signature  = Base64(HMAC-SHA256(apiSecret, prehash))
passphrase = Base64(HMAC-SHA256(apiSecret, rawPassphrase))
```

**Headers:**
```
KC-API-KEY: <apiKey>
KC-API-SIGN: <signature>
KC-API-TIMESTAMP: <timestamp>
KC-API-PASSPHRASE: <signedPassphrase>
KC-API-KEY-VERSION: 2
```

**Ендпоінти:**
| Дія | URL |
|---|---|
| Баланси | `GET /api/v1/accounts?type=trade` |
| Угоди (fills) | `GET /api/v1/fills` |
| Ціни | `GET /api/v1/market/allTickers` |

**Особливості:**
- KuCoin не має публічного USDT futures → `GetOpenPositions()` повертає `[]`
- `KC-API-KEY-VERSION: 2` — обов'язковий для підписаної passphrase
- Не підтримує Trader і SpotTrader інтерфейси

---

## Додавання нової біржі

1. Створити файл `internal/services/exchange/newexchange.go`
2. Реалізувати `Exchange` інтерфейс (або також `Trader` і/або `SpotTrader`)
3. Додати compile-time check:
   ```go
   var _ exchange.Exchange = (*NewExchange)(nil)
   ```
4. Зареєструвати в `registry.go`:
   ```go
   func Registry() map[string]Exchange {
       return map[string]Exchange{
           "newexchange": NewNewExchange(),
           // ...
       }
   }
   ```
5. Якщо підтримує торгівлю — додати в `placeOrderRequest.validate` в `handlers/order.go`:
   ```go
   Exchange string `json:"exchange" validate:"required,oneof=binance okx bybit newexchange"`
   ```
6. Якщо підтримує SpotTrader — додати compile-time check:
   ```go
   var _ exchange.SpotTrader = (*NewExchange)(nil)
   ```

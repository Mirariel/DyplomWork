# TradeTracker Go — Exchange Adapters

Всі адаптери знаходяться в `internal/services/exchange/`.

---

## Інтерфейси

### Exchange (базовий)
```go
type Exchange interface {
    GetBalances(creds Credentials) ([]Balance, error)
    GetPositions(creds Credentials) ([]Position, error)
    GetClosedTrades(creds Credentials, since time.Time) ([]ClosedTrade, error)
    GetLivePrices(symbols []string) (map[string]float64, error)
}
```

### Trader (опціональний — тільки Binance, OKX, Bybit)
```go
type Trader interface {
    PlaceOrder(creds Credentials, req PlaceOrderRequest) (PlacedOrder, error)
    CancelOrder(creds Credentials, req CancelOrderRequest) error
    GetOrderStatus(creds Credentials, req CancelOrderRequest) (PlacedOrder, error)
}
```

Перевірка наявності Trader:
```go
if trader, ok := ex.(exchange.Trader); ok {
    // ця біржа підтримує торгівлю
}
```

### Credentials
```go
type Credentials struct {
    APIKey     string
    APISecret  string
    Passphrase string  // тільки OKX
}
```

---

## Підтримувані біржі

| Біржа | Читання | Торгівля | Підпис |
|---|---|---|---|
| Binance | Spot + Futures | так | HMAC-SHA256 query string |
| OKX | Spot + Futures + Earn | так | HMAC-SHA256 Base64 + passphrase |
| Bybit | UTA (spot + futures) | так | HMAC-SHA256 body string |
| Gate.io | Spot + USDT-M Futures | ні | HMAC-SHA512 |
| Kraken | Spot | ні | HMAC-SHA512 + Base64 decode secret |
| KuCoin | Spot | ні | HMAC-SHA256 Base64 |

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
| Spot ордери | `GET /api/v3/myTrades` |
| Futures угоди | `GET /fapi/v1/userTrades` |
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
passphrase = Base64(HMAC-SHA256(apiSecret, rawPassphrase))  // для KC-API-KEY-VERSION: 2
```

**Headers:**
```
OK-ACCESS-KEY: <apiKey>
OK-ACCESS-SIGN: <signature>
OK-ACCESS-TIMESTAMP: <timestamp>
OK-ACCESS-PASSPHRASE: <signedPassphrase>
```

**Ендпоінти:**
| Дія | URL |
|---|---|
| Баланси | `GET /api/v5/account/balance` |
| Funding | `GET /api/v5/asset/balances` |
| Earn | `GET /api/v5/finance/savings/balance` |
| Позиції | `GET /api/v5/account/positions` |
| Закриті угоди | `GET /api/v5/trade/fills-history` |
| Ціни | `GET /api/v5/market/tickers?instType=SPOT` |
| PlaceOrder | `POST /api/v5/trade/order` |
| CancelOrder | `POST /api/v5/trade/cancel-order` |

**Особливості:**
- `instId` для spot: `"BTC-USDT"`, для futures: `"BTC-USDT-SWAP"`
- `tdMode`: `"cash"` (spot), `"cross"` (futures)
- Помилки ордерів перевіряти в `resp.Data[0].SCode` (не в top-level `code`)

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
- Gate не підтримує Trader інтерфейс (немає PlaceOrder)

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
- Kraken не має публічного API futures → `GetPositions()` повертає `[]`
- `GetLivePrices()` повертає `{}` (Binance/OKX покривають ціни)
- Нормалізація активів: `XXBT` → `BTC`, `ZUSD` → `USD`, `XETH` → `ETH`
- Використовує `postForm` (не `postJSON`)

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
- KuCoin не має публічного USDT futures → `GetPositions()` повертає `[]`
- `KC-API-KEY-VERSION: 2` — обов'язковий для підписаної passphrase

---

## Додавання нової біржі

1. Створити файл `internal/services/exchange/newexchange.go`
2. Реалізувати `Exchange` інтерфейс (або також `Trader`)
3. Додати compile-time check:
   ```go
   var _ Exchange = (*NewExchange)(nil)
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

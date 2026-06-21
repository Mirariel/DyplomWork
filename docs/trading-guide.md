# TradeTracker Go — Керівництво з торгівлі

## Підтримувані біржі

| Біржа | Синк | Торгівля | Боти |
|---|---|---|---|
| Binance | так | так | так |
| OKX | так | так | так |
| Bybit | так | так | так |
| Gate.io | так | ні | ні |
| Kraken | так | ні | ні |
| KuCoin | так | ні | ні |

---

## Налаштування API ключів

### 1. Підключити ключі

```bash
curl -X POST http://localhost:8080/api/portfolio/credentials \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":   "binance",
    "label":      "Main account",
    "api_key":    "your-binance-api-key",
    "api_secret": "your-binance-api-secret"
  }'
```

Для OKX і KuCoin додати `"passphrase"`:
```json
{
  "exchange":   "okx",
  "api_key":    "...",
  "api_secret": "...",
  "passphrase": "your-okx-passphrase"
}
```

### 2. Необхідні права API ключа

| Біржа | Потрібні дозволи |
|---|---|
| Binance | `Read Info` + `Enable Spot & Margin Trading` |
| OKX | `Read` + `Trade` |
| Bybit | `Read-Write` → `Orders` + `Positions` |

**Ніколи не вмикати `Withdrawal` — це небезпечно.**

---

## Звичайні ордери

### Market ордер

```bash
curl -X POST http://localhost:8080/api/orders \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":  "bybit",
    "symbol":    "BTC",
    "side":      "buy",
    "type":      "market",
    "category":  "spot",
    "quantity":  0.001
  }'
```

### Limit ордер

```bash
curl -X POST http://localhost:8080/api/orders \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":  "binance",
    "symbol":    "ETH",
    "side":      "buy",
    "type":      "limit",
    "category":  "spot",
    "quantity":  0.1,
    "price":     3000
  }'
```

### Futures ордер

```bash
curl -X POST http://localhost:8080/api/orders \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":  "okx",
    "symbol":    "BTC",
    "side":      "buy",
    "type":      "market",
    "category":  "futures",
    "quantity":  0.01
  }'
```

### Управління ордерами

```bash
# Перевірити статус (живий запит до біржі)
curl http://localhost:8080/api/orders/42 -H "Authorization: Bearer <token>"

# Всі ордери / фільтр по статусу
curl "http://localhost:8080/api/orders?status=new" -H "Authorization: Bearer <token>"

# Скасувати
curl -X DELETE http://localhost:8080/api/orders/42 -H "Authorization: Bearer <token>"
```

### Статуси ордерів

| Статус | Опис |
|---|---|
| `new` | Прийнятий біржею, чекає виконання |
| `partial` | Частково виконаний |
| `filled` | Повністю виконаний |
| `cancelled` | Скасований |
| `rejected` | Відхилений біржею (помилка або недостатньо коштів) |

### Lifecycle ордеру

```
POST /api/orders → Валідація → Decrypt credentials
    → INSERT orders (status=new) — запис завжди є в БД
    → PlaceOrder на біржі
        ├── Успіх → UpdateStatus(exchange_order_id, "new")
        └── Помилка → MarkFailed("rejected", errMsg) → 502
```

---

## Smart Orders

Умовні ордери що автоматично виконуються при досягненні ціни. Перевірка кожні **5 секунд**.

### Stop-Loss

Захист від збитків — продає при падінні ціни нижче `trigger_price`.

```bash
curl -X POST http://localhost:8080/api/smart-orders \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":      "binance",
    "symbol":        "BTC",
    "side":          "sell",
    "type":          "stop_loss",
    "quantity":      0.001,
    "trigger_price": 60000
  }'
```

### Take-Profit

Фіксація прибутку — продає при зростанні ціни вище `trigger_price`.

```bash
curl -X POST http://localhost:8080/api/smart-orders \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":      "binance",
    "symbol":        "BTC",
    "side":          "sell",
    "type":          "take_profit",
    "quantity":      0.001,
    "trigger_price": 75000
  }'
```

### Trailing Stop

Стоп що слідує за ціною — виконується коли ціна відхилилась від піку на `trail_delta` USD.

```bash
curl -X POST http://localhost:8080/api/smart-orders \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":    "bybit",
    "symbol":      "ETH",
    "side":        "sell",
    "type":        "trailing_stop",
    "quantity":    0.5,
    "trail_delta": 200
  }'
```

При `trail_delta=200`: якщо ETH досяг $3500 (пік), ордер спрацює при падінні до $3300.

### Управління smart orders

```bash
# Список
curl http://localhost:8080/api/smart-orders -H "Authorization: Bearer <token>"

# Скасувати
curl -X DELETE http://localhost:8080/api/smart-orders/7 -H "Authorization: Bearer <token>"
```

### Статуси smart order

| Статус | Опис |
|---|---|
| `active` | Моніториться, чекає trigger |
| `triggered` | Спрацював — виставлений market ордер |
| `cancelled` | Скасований вручну |
| `failed` | Помилка при виставленні ордеру (Telegram сповіщення) |

---

## Grid Bot

Бот автоматично купує на нижніх рівнях сітки і продає на верхніх. Прибуток від волатильності в діапазоні.

### Створити і запустити

```bash
# Створити
curl -X POST http://localhost:8080/api/bots \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":    "binance",
    "symbol":      "BTC",
    "category":    "spot",
    "lower_price": 60000,
    "upper_price": 70000,
    "grids":       10,
    "total_usdt":  1000
  }'

# Запустити (виставляє buy-ордери по всіх рівнях)
curl -X POST http://localhost:8080/api/bots/1/start \
  -H "Authorization: Bearer <token>"
```

З параметрами вище: 10 рівнів у діапазоні $60k–$70k, по $100 USDT на кожен.
Крок між рівнями: ($70k − $60k) / 10 = $1000.

### Логіка роботи

```
При старті: виставити buy-ордери на всіх рівнях нижче поточної ціни
Кожні 10с: перевіряти виконані (filled) ордери
    → якщо buy filled: виставити sell на наступному рівні вгору
    → якщо sell filled: виставити buy на попередньому рівні вниз
```

### Управління

```bash
curl -X POST http://localhost:8080/api/bots/1/stop -H "Authorization: Bearer <token>"
curl -X DELETE http://localhost:8080/api/bots/1 -H "Authorization: Bearer <token>"
```

---

## DCA Bot

Dollar-Cost Averaging — регулярна купівля активу за розкладом незалежно від ціни.

### Створити і запустити

```bash
# Купувати BTC на $100 USDT кожні 24 години
curl -X POST http://localhost:8080/api/dca \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":       "binance",
    "symbol":         "BTC",
    "amount_usd":     100,
    "interval_hours": 24
  }'

# Запустити (з негайною першою покупкою)
curl -X POST "http://localhost:8080/api/dca/1/start?buy_now=true" \
  -H "Authorization: Bearer <token>"
```

### Логіка роботи

```
Кожні 5 хвилин: перевіряти DCA боти де next_buy_at <= now()
    → qty = amount_usd / live_price
    → PlaceOrder(market buy, qty)
    → total_invested += amount_usd, orders_placed++
    → next_buy_at = now() + interval_hours
```

### Моніторинг

```bash
curl http://localhost:8080/api/dca/1 -H "Authorization: Bearer <token>"
```

```json
{
  "id": 1,
  "symbol": "BTC",
  "amount_usd": 100,
  "interval_hours": 24,
  "total_invested": 700.0,
  "orders_placed": 7,
  "next_buy_at": "2026-06-22T10:00:00Z",
  "status": "running"
}
```

---

## Символи (symbol)

Передається **базовий тікер** без квотованої валюти:

| Передати | Binance | OKX | Bybit |
|---|---|---|---|
| `BTC` | `BTCUSDT` | `BTC-USDT` | `BTCUSDT` |
| `ETH` | `ETHUSDT` | `ETH-USDT` | `ETHUSDT` |
| `SOL` | `SOLUSDT` | `SOL-USDT` | `SOLUSDT` |

Для futures OKX: `BTC` → `BTC-USDT-SWAP`

---

## Обробка помилок

### 400 — немає активних credentials
```json
{ "error": "no active credentials found for binance" }
```
Рішення: `POST /api/portfolio/credentials`.

### 422 — помилка валідації
```json
{ "error": "Exchange: oneof; Price: gt" }
```

### 502 — помилка API біржі
```json
{ "error": "binance order API: Account has insufficient balance" }
```
Ордер збережений зі статусом `rejected`. Перевірте баланс.

---

## Telegram сповіщення

Якщо задані `TELEGRAM_BOT_TOKEN` і `TELEGRAM_CHAT_ID` в `.env` — сервер надсилає сповіщення при:
- Smart Order triggered / failed
- DCA Bot виконав покупку
- Grid Bot помилка

Налаштування: `POST /api/portfolio/credentials` не потрібне — береться з `.env`.

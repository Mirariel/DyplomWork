# TradeTracker Go — Керівництво з торгівлі

## Підтримувані біржі

Торгівля (PlaceOrder) доступна для: **Binance**, **OKX**, **Bybit**.

Gate.io, Kraken, KuCoin — тільки читання (синк балансів і угод).

---

## Налаштування API ключів

### 1. Підключити ключі

```bash
curl -X POST http://localhost:8080/api/portfolio/credentials \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "exchange":   "binance",
    "api_key":    "your-binance-api-key",
    "api_secret": "your-binance-api-secret"
  }'
```

Для OKX додати `"passphrase"`:
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

## Розміщення ордерів

### Market ордер (негайне виконання за ринковою ціною)

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

### Limit ордер (виконання за вказаною ціною або краще)

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

---

## Управління ордерами

### Перевірити статус ордеру

Сервер робить **живий запит до біржі** і оновлює БД:
```bash
curl http://localhost:8080/api/orders/42 \
  -H "Authorization: Bearer <token>"
```

**Відповідь:**
```json
{
  "id": 42,
  "exchange_order_id": "3847291",
  "symbol": "ETH",
  "side": "buy",
  "type": "limit",
  "status": "partial",
  "filled_qty": 0.05,
  "avg_price": 2998.5,
  "quantity": 0.1,
  "price": 3000
}
```

### Список ордерів

```bash
# Всі ордери
curl "http://localhost:8080/api/orders" -H "Authorization: Bearer <token>"

# Тільки відкриті
curl "http://localhost:8080/api/orders?status=new" -H "Authorization: Bearer <token>"

# Заповнені
curl "http://localhost:8080/api/orders?status=filled" -H "Authorization: Bearer <token>"
```

### Скасувати ордер

```bash
curl -X DELETE http://localhost:8080/api/orders/42 \
  -H "Authorization: Bearer <token>"
```

**Відповідь:**
```json
{ "status": "cancelled", "id": 42 }
```

---

## Статуси ордерів

| Статус | Опис |
|---|---|
| `new` | Ордер прийнятий біржею, чекає на виконання |
| `partial` | Частково виконаний |
| `filled` | Повністю виконаний |
| `cancelled` | Скасований (вручну або при закритті ринку) |
| `rejected` | Відхилений біржею (помилка, недостатньо коштів тощо) |

---

## Lifecycle ордеру

```
POST /api/orders
    │
    ▼
Валідація запиту
    │
    ▼
Отримати credentials з БД (розшифрувати)
    │
    ▼
Зберегти в orders (status="new")  ← запис ЗАВЖДИ є в БД
    │
    ▼
PlaceOrder на біржі
    ├── Успіх → UpdateStatus(exchange_order_id, "new")
    └── Помилка → MarkFailed("rejected", errMsg) → 502
    │
    ▼
Повернути Order зі статусом
```

Якщо запит до біржі впав — ордер залишається в БД зі статусом `rejected` і `error_message`.

---

## Обробка помилок

### 400 — немає активних credentials
```json
{ "error": "no active credentials found for binance" }
```
Рішення: додати ключі через `POST /api/portfolio/credentials`.

### 422 — помилка валідації
```json
{ "error": "Exchange: oneof; Price: gt" }
```
Рішення: перевірити поля запиту.

### 502 — помилка API біржі
```json
{ "error": "binance order API: Account has insufficient balance" }
```
Ордер збережений в БД зі статусом `rejected`. Перевірте баланс.

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

## Майбутні можливості (Roadmap)

- **Stop-Loss / Take-Profit** — умовні ордери
- **Trailing Stop** — стоп що слідує за ціною
- **Grid Bot** — автоматична купівля/продаж в діапазоні
- **DCA Bot** — регулярні покупки за розкладом
- **TWAP** — розбиття великого ордеру на дрібні

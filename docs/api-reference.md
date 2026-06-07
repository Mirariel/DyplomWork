# TradeTracker Go — API Reference

Base URL: `http://localhost:8080`

## Автентифікація

JWT токен передається двома способами:
- **Header:** `Authorization: Bearer <token>`
- **Cookie:** `token=<token>` (встановлюється при логіні)

Токен живе **24 години**. Після логіну повертається в тілі відповіді та Cookie.

---

## Public endpoints

### POST /api/auth/register
Реєстрація нового користувача.

**Rate limit:** 10 запитів/хвилину per IP.

**Тіло:**
```json
{
  "username": "alice",
  "email": "alice@example.com",
  "password": "secret123"
}
```

**Відповідь 201:**
```json
{
  "token": "eyJ...",
  "user": { "id": 1, "username": "alice", "email": "alice@example.com" }
}
```

**Помилки:** 409 (email/username вже зайнятий), 422 (валідація)

---

### POST /api/auth/login
**Тіло:**
```json
{ "email": "alice@example.com", "password": "secret123" }
```

**Відповідь 200:**
```json
{ "token": "eyJ...", "user": { "id": 1, "username": "alice" } }
```

**Помилки:** 401 (невірний пароль), 404 (не знайдено)

---

### POST /api/auth/logout
Очищає cookie `token`. Відповідь 200: `{"message": "logged out"}`.

---

### GET /health
```json
{ "status": "ok", "version": "0.1.0" }
```

---

## Protected endpoints (JWT required)

### GET /api/auth/me
Повертає профіль поточного користувача.

**Відповідь 200:**
```json
{ "id": 1, "username": "alice", "email": "alice@example.com", "created_at": "..." }
```

---

## Portfolio

### GET /api/portfolio/
Повний портфель: активи, відкриті позиції, остання торгова активність.

**Відповідь 200:**
```json
{
  "assets": [
    { "id": 1, "symbol": "BTC", "exchange": "binance", "balance": 0.5,
      "price": 67420, "value_usd": 33710 }
  ],
  "positions": [
    { "id": 2, "symbol": "ETH", "side": "LONG", "exchange": "bybit",
      "quantity": 1.0, "entry_price": 3100, "mark_price": 3210,
      "unrealized_pnl": 110, "pnl_pct": 3.55, "leverage": "10x" }
  ],
  "history": [...],
  "total_value": 50000.0
}
```

---

### GET /api/portfolio/history?limit=15&offset=0
Список закритих угод.

| Параметр | Тип | За замовчуванням |
|---|---|---|
| limit | int | 15 |
| offset | int | 0 |

---

### GET /api/portfolio/credentials
Список підключених бірж (без розшифрованих ключів).

**Відповідь 200:**
```json
[
  { "id": 1, "exchange": "binance", "is_active": true, "created_at": "..." }
]
```

---

### POST /api/portfolio/credentials
Підключити або оновити API ключі біржі.

**Тіло:**
```json
{
  "exchange":   "binance",
  "api_key":    "your-api-key-min-16-chars",
  "api_secret": "your-api-secret-min-16-chars",
  "passphrase": ""
}
```
Поле `passphrase` обов'язкове тільки для OKX.

**Відповідь 201:** щойно створений запис (без ключів у відповіді).

---

### DELETE /api/portfolio/credentials/:id
Видалити підключену біржу.

**Відповідь 200:** `{"message": "deleted"}`

---

### PATCH /api/positions/:id/comment
### PATCH /api/history/:id/comment
Додати/змінити коментар до позиції або закритої угоди.

**Тіло:** `{ "comment": "Хеджувальна позиція" }`

---

## Sync

Ендпоінти запускають синхронізацію вручну. Автоматичний синк відбувається кожні 15 хвилин.

### POST /api/sync/full
Повний синк: баланси + позиції + закриті угоди (7 днів).

**Відповідь 200:**
```json
{ "synced": ["binance", "bybit"], "errors": { "okx": "invalid api key" } }
```

---

### POST /api/sync/positions
Тільки відкриті позиції (швидше).

---

### POST /api/sync/history?days=7
Закриті угоди за останні N днів.

---

### GET /api/sync/prices
Оновити ціни для всіх активів з кешу.

---

## Orders

Тільки для бірж Binance, OKX, Bybit.

### POST /api/orders
Розмістити ордер.

**Тіло:**
```json
{
  "exchange":  "binance",
  "symbol":    "BTC",
  "side":      "buy",
  "type":      "limit",
  "category":  "spot",
  "quantity":  0.001,
  "price":     65000
}
```

| Поле | Тип | Обов'язкове | Допустимі значення |
|---|---|---|---|
| exchange | string | так | binance, okx, bybit |
| symbol | string | так | BTC, ETH, SOL... |
| side | string | так | buy, sell |
| type | string | так | market, limit |
| category | string | так | spot, futures |
| quantity | float | так | > 0 |
| price | float | для limit | > 0 |

**Відповідь 201:**
```json
{
  "id": 42,
  "exchange": "binance",
  "exchange_order_id": "3847291",
  "symbol": "BTC",
  "side": "buy",
  "type": "limit",
  "category": "spot",
  "quantity": 0.001,
  "price": 65000,
  "filled_qty": 0,
  "avg_price": 0,
  "status": "new",
  "created_at": "2025-01-15T10:00:00Z"
}
```

**Помилки:**
- 400 — немає активних credentials для цієї біржі
- 422 — помилка валідації
- 502 — помилка API біржі (ордер збережений зі статусом `rejected`)

---

### GET /api/orders
Список ордерів (останні 100).

**Параметри:**
| Параметр | Тип | Опис |
|---|---|---|
| status | string | фільтр: new, partial, filled, cancelled, rejected |

**Відповідь 200:** масив Order об'єктів.

---

### GET /api/orders/:id
Статус ордеру (живий запит до біржі, якщо є exchange_order_id).

**Відповідь 200:** Order об'єкт з актуальним статусом.

---

### DELETE /api/orders/:id
Скасувати ордер.

**Відповідь 200:** `{ "status": "cancelled", "id": 42 }`

**Помилки:**
- 404 — ордер не знайдений (або чужий)
- 409 — ордер вже filled або cancelled
- 502 — помилка API біржі

---

## WebSocket

**URL:** `ws://localhost:8080/ws`

### Протокол підключення

```
1. Клієнт підключається
2. Клієнт надсилає auth-повідомлення
3. Сервер підтверджує
4. Сервер надсилає оновлення кожні 2 секунди
```

**1. Auth:**
```json
{ "type": "auth", "token": "eyJ..." }
```

**2. Підтвердження:**
```json
{ "type": "auth_success", "user_id": 7 }
```

**3. Повідомлення оновлення (кожні 2s):**
```json
{
  "type": "update",
  "positions": [
    {
      "symbol": "BTC",
      "side": "LONG",
      "exchange": "bybit",
      "entry_price": 65000,
      "mark_price": 67420,
      "pnl": 241.5,
      "pnl_pct": 1.85,
      "leverage": "10x"
    }
  ],
  "spot_prices": {
    "BTC": 67420,
    "ETH": 3210,
    "SOL": 145.0
  }
}
```

Якщо auth не надіслано протягом 10 секунд — з'єднання закривається.

---

## Коди помилок

| Код | Значення |
|---|---|
| 400 | Некоректний запит (invalid body, немає credentials) |
| 401 | Потрібна автентифікація |
| 404 | Ресурс не знайдений |
| 409 | Конфлікт стану (ордер вже виконаний) |
| 422 | Помилка валідації полів |
| 429 | Перевищено rate limit |
| 500 | Помилка сервера |
| 502 | Помилка зовнішнього API біржі |

**Формат помилки:**
```json
{ "error": "price is required for limit orders" }
```

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

### GET /metrics
Prometheus метрики (text/plain format).

Включає: HTTP request counter/histogram, WS clients gauge, active bots/smart-orders gauges, scheduler job duration histogram.

---

## Protected endpoints (JWT required)

### GET /api/auth/me
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
| from | string (ISO date) | — |
| to | string (ISO date) | — |

Кожен запис включає поле `max_size` — розмір позиції в USD (notionalUsd від OKX або qty×entryPrice як fallback).

---

### GET /api/portfolio/credentials
Список підключених бірж (без розшифрованих ключів).

**Відповідь 200:**
```json
[{
  "id": 1,
  "exchange": "binance",
  "label": "Main account",
  "api_key_hint": "abcd••••uvwx",
  "has_passphrase": false,
  "is_active": true,
  "last_sync_at": "2026-06-21T10:00:00Z",
  "last_sync_error": null,
  "created_at": "2026-06-20T08:00:00Z"
}]
```

---

### POST /api/portfolio/credentials
**Тіло:**
```json
{
  "exchange":   "binance",
  "label":      "Main account",
  "api_key":    "your-api-key-min-16-chars",
  "api_secret": "your-api-secret-min-16-chars",
  "passphrase": ""
}
```
`label` — довільна мітка для зручності (необов'язкове).
`passphrase` обов'язковий тільки для OKX і KuCoin.
`api_key` та `api_secret` — мінімум 16 символів.

**Відповідь 201:** щойно створений запис; `api_key_hint` = перші 4 + `••••` + останні 4 символи ключа.

---

### DELETE /api/portfolio/credentials/:id
**Відповідь 200:** `{"message": "deleted"}`

---

### PATCH /api/positions/:id/comment
### PATCH /api/history/:id/comment
**Тіло:** `{ "comment": "Хеджувальна позиція" }`

---

### PATCH /api/portfolio/assets/:id/price
Вручну оновити середню ціну покупки активу.

**Тіло:**
```json
{ "avg_buy_price": 45000.0, "manually_set": true }
```

---

## Sync

Автоматичний синк — кожні 15 хвилин. Ендпоінти для ручного запуску.

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
  "id": 42, "exchange": "binance", "exchange_order_id": "3847291",
  "symbol": "BTC", "side": "buy", "type": "limit", "category": "spot",
  "quantity": 0.001, "price": 65000, "filled_qty": 0, "avg_price": 0,
  "status": "new", "created_at": "2026-06-21T10:00:00Z"
}
```

**Помилки:** 400 (немає credentials), 422 (валідація), 502 (помилка біржі → status=rejected)

---

### GET /api/orders?status=new
Список ордерів (останні 100). Фільтр: `new | partial | filled | cancelled | rejected`.

### GET /api/orders/:id
Статус ордеру (живий запит до біржі якщо є `exchange_order_id`).

### DELETE /api/orders/:id
Скасувати ордер. **Відповідь 200:** `{ "status": "cancelled", "id": 42 }`

---

## Smart Orders

Умовні ордери: Stop-Loss, Take-Profit, Trailing Stop. Моніторяться кожні 5 секунд.

### POST /api/smart-orders
**Тіло:**
```json
{
  "exchange":    "binance",
  "symbol":      "BTC",
  "side":        "sell",
  "type":        "stop_loss",
  "quantity":    0.001,
  "trigger_price": 60000
}
```

| Поле | Тип | Допустимі значення |
|---|---|---|
| type | string | stop_loss, take_profit, trailing_stop |
| side | string | buy, sell |
| trigger_price | float | > 0 (для SL/TP) |
| trail_delta | float | > 0 (для trailing_stop — відхилення від піку в USD) |

**Відповідь 201:**
```json
{
  "id": 7, "exchange": "binance", "symbol": "BTC", "side": "sell",
  "type": "stop_loss", "quantity": 0.001, "trigger_price": 60000,
  "status": "active", "created_at": "2026-06-21T10:00:00Z"
}
```

---

### GET /api/smart-orders
Список всіх smart orders поточного користувача.

### GET /api/smart-orders/:id
Деталі одного smart order.

### DELETE /api/smart-orders/:id
Скасувати (status → cancelled).

---

**Статуси smart order:**

| Статус | Опис |
|---|---|
| `active` | Моніториться, чекає на trigger |
| `triggered` | Спрацював — виставлений ринковий ордер |
| `cancelled` | Скасований вручну |
| `failed` | Помилка при виставленні ордеру |

---

## Grid Bots

Бот автоматично купує/продає в заданому діапазоні цін. Перевірка кожні 10 секунд.

### POST /api/bots
**Тіло:**
```json
{
  "exchange":   "binance",
  "symbol":     "BTC",
  "category":   "spot",
  "lower_price": 60000,
  "upper_price": 70000,
  "grids":      10,
  "total_usdt": 1000
}
```

| Поле | Опис |
|---|---|
| grids | Кількість рівнів сітки (2–100) |
| total_usdt | Загальний бюджет в USDT |
| lower_price / upper_price | Діапазон цін сітки |

**Відповідь 201:** об'єкт бота зі статусом `stopped`.

---

### GET /api/bots
Список ботів поточного користувача.

### GET /api/bots/:id
Деталі бота + grid рівні.

### POST /api/bots/:id/start
Запустити бота (status → running, виставляє buy-ордери по всіх рівнях).

### POST /api/bots/:id/stop
Зупинити бота (status → stopped, скасовує відкриті ордери).

### DELETE /api/bots/:id
Видалити бота (можна тільки зупиненого).

---

**Статуси бота:**

| Статус | Опис |
|---|---|
| `stopped` | Не активний |
| `running` | Активний, моніторить ціну |
| `error` | Помилка при виставленні ордеру |

---

## DCA Bots

Dollar-Cost Averaging: автоматична купівля активу за розкладом.

### POST /api/dca
**Тіло:**
```json
{
  "exchange":    "binance",
  "symbol":      "BTC",
  "amount_usd":  100,
  "interval_hours": 24
}
```

| Поле | Опис |
|---|---|
| amount_usd | Сума в USDT на одну покупку |
| interval_hours | Інтервал між покупками (в годинах) |

**Відповідь 201:** об'єкт DCA бота зі статусом `stopped`.

---

### GET /api/dca
Список DCA ботів.

### GET /api/dca/:id
Деталі: total_invested, orders_placed, next_buy_at.

### POST /api/dca/:id/start?buy_now=true
Запустити бота. `?buy_now=true` — виконати покупку негайно при старті.

### POST /api/dca/:id/stop
Зупинити бота.

### DELETE /api/dca/:id
Видалити бота.

---

## Analytics

### GET /api/analytics/summary
Торгова статистика по всіх закритих угодах.

| Параметр | Тип | Опис |
|---|---|---|
| days | int | Останні N днів (0 = всі) |
| exchange | string | Фільтр по біржі |
| from | string (ISO date) | Початок діапазону |
| to | string (ISO date) | Кінець діапазону |

**Відповідь 200:**
```json
{
  "total_trades": 42,
  "winning_trades": 28,
  "losing_trades": 14,
  "winrate": 66.7,
  "total_realized_pnl": 1250.5,
  "avg_pnl": 29.77,
  "best_trade": 450.0,
  "worst_trade": -120.0,
  "avg_win": 89.3,
  "avg_loss": -42.5,
  "profit_factor": 2.1,
  "total_fees": 38.5
}
```

---

### GET /api/analytics/coins
Статистика по кожному активу.

**Відповідь 200:**
```json
[
  {
    "symbol": "BTC", "exchange": "binance",
    "total_trades": 15, "winning_trades": 11,
    "winrate": 73.3, "total_pnl": 800.0,
    "avg_pnl": 53.3, "best_trade": 450.0, "worst_trade": -120.0
  }
]
```

---

### GET /api/analytics/snapshots?days=30
Знімки портфеля за останні N днів.

| Параметр | Тип | Опис |
|---|---|---|
| days | int | Останні N днів |
| hours | int | Останні N годин |
| from | string (ISO) | Початок діапазону |
| to | string (ISO) | Кінець діапазону |

**Відповідь 200:**
```json
[
  { "id": 1, "user_id": 7, "total_value": 48500.0, "spot_value": 30000.0,
    "futures_pnl": -200.0, "snapshot_at": "2026-06-20T10:00:00Z" },
  { "id": 2, "user_id": 7, "total_value": 50000.0, "spot_value": 32000.0,
    "futures_pnl": 150.0,  "snapshot_at": "2026-06-21T10:00:00Z" }
]
```

---

### POST /api/analytics/snapshot
Зробити знімок портфеля вручну (автоматично — щогодини).

**Відповідь 200:** `{"message": "snapshot saved"}`

---

### GET /api/analytics/chart?range=7d&exchange=all
Дані для побудови графіку вартості портфеля (15-хвилинні бакети).

| Параметр | Тип | Допустимі значення |
|---|---|---|
| range | string | `1d`, `7d`, `30d`, `90d`, `custom` |
| exchange | string | `all`, `binance`, `okx`, `bybit`... |
| from | string (ISO) | для `range=custom` |
| to | string (ISO) | для `range=custom` |

**Відповідь 200:**
```json
[
  { "recorded_at": "2026-06-20T10:00:00Z", "value": 48500.0 },
  { "recorded_at": "2026-06-20T10:15:00Z", "value": 48720.0 }
]
```

---

### GET /api/analytics/arbitrage?min_spread=0.5
Арбітражні можливості між біржами.

| Параметр | Тип | За замовчуванням |
|---|---|---|
| min_spread | float | 0.5 (%) |

**Відповідь 200:**
```json
[
  {
    "symbol": "ETH",
    "buy_exchange": "kraken",
    "sell_exchange": "binance",
    "buy_price": 3190.0,
    "sell_price": 3210.0,
    "spread_pct": 0.63
  }
]
```

---

## Futures Positions

### GET /api/futures/positions
Відкриті ф'ючерсні позиції (з БД, оновлюються при синку).

| Параметр | Тип | Опис |
|---|---|---|
| exchange | string | Фільтр по біржі (необов'язково) |

**Відповідь 200:**
```json
{
  "positions": [
    {
      "id": 1, "user_id": 7, "exchange": "okx",
      "symbol": "BTCUSDT", "side": "LONG",
      "size": 0.01, "entry_price": 65000, "mark_price": 67000,
      "unrealized_pnl": 20.0, "leverage": 10,
      "margin_type": "cross", "synced_at": "2026-06-24T10:00:00Z"
    }
  ],
  "total_unrealized_pnl": 20.0
}
```

---

## Spot Trades

### GET /api/spot-trades
Список спот-угод (з БД, синхронізуються при `/sync/history`).

| Параметр | Тип | Опис |
|---|---|---|
| exchange | string | Фільтр по біржі (необов'язково) |

**Відповідь 200:**
```json
{
  "trades": [
    {
      "id": 1, "user_id": 7, "exchange": "okx",
      "symbol": "BTCUSDT", "side": "buy",
      "quantity": 0.001, "price": 65000,
      "fee": 0.065, "fee_asset": "USDT",
      "traded_at": 1719216000000
    }
  ]
}
```

---

## Market Data

### GET /api/market/top-symbols
Список топ-символів за обсягом (для WS-стрічки цін).

**Відповідь 200:** `["BTCUSDT", "ETHUSDT", "SOLUSDT", ...]`

---

## AI Advisor

### POST /api/ai/ask
Запит до AI-радника (аналіз ринку, стратегії, Q&A). Використовує Claude API.

**Тіло:**
```json
{
  "message": "Яка твоя думка про поточну ситуацію з BTC?",
  "history": [
    { "role": "user", "content": "попереднє питання" },
    { "role": "assistant", "content": "попередня відповідь" }
  ]
}
```

**Відповідь 200:**
```json
{ "reply": "На мою думку, BTC зараз..." }
```

---

## WebSocket

**URL:** `ws://localhost:8080/ws`

### Протокол

```
1. Клієнт підключається
2. Клієнт надсилає { "type": "auth", "token": "eyJ..." }
3. Сервер відповідає { "type": "auth_success", "user_id": 7 }
4. Сервер надсилає update кожні 2 секунди
```

Якщо auth не надіслано протягом 10 секунд — з'єднання закривається.

**Повідомлення оновлення (кожні 2s):**
```json
{
  "type": "update",
  "positions": [
    {
      "symbol": "BTC", "side": "LONG", "exchange": "bybit",
      "entry_price": 65000, "mark_price": 67420,
      "pnl": 241.5, "pnl_pct": 1.85, "leverage": "10x"
    }
  ],
  "spot_prices": {
    "BTC": 67420,
    "ETH": 3210,
    "SOL": 145.0
  }
}
```

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

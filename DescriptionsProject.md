# TradeTracker Go — CLAUDE.md

Цей файл є живою документацією проєкту для Claude Code.
Оновлювати після кожної значимої зміни архітектури або завершення фази.

---

## Контекст і ціль

**Що це:** Переписування PHP-прототипу (`C:\wamp64\www\kursova`) на Go з метою перетворити
його на повноцінний трейдинг-продукт рівня [Bitsgap](https://bitsgap.com) —
мультибіржовий портфельний трекер з торговими ботами, smart orders та аналітикою.

**Чому Go:** Криптотрейдинг — це latency-sensitive домен. PHP з синхронним виконанням
не масштабується до сотень WebSocket-з'єднань і паралельних API-запитів до 10+ бірж.
Go дає нам goroutines, канали, і compile-time safety без overhead JVM/Node.

**PHP-прототип:** `kursova/` — курсова робота, живе окремо. Використовується лише як
**референс логіки** (адаптери бірж, схема БД, бізнес-правила синку). Дані не мігруємо.

---

## Поточний стан

### Фаза 0 — Foundation ✅ DONE
### Фаза 1 — Міграції + Перший живий запуск ✅ DONE
### Фаза 1 (залишок) — Rate limiting, slog, graceful shutdown ✅ DONE
### Фаза 2 — Production-ready backend ✅ DONE
### Фаза 3 — Торгівля (Binance + OKX + Bybit PlaceOrder/Cancel/Status) ✅ DONE
### Фаза 4 — Smart Orders (Stop-Loss, Take-Profit, Trailing Stop) ✅ DONE
### Фаза 4 (залишок) — Grid Bot ✅ DONE

**Сервер зараз:** `http://localhost:8080` (Go + Fiber v2)
**WebSocket:** `ws://localhost:8080/ws`
**БД:** MySQL `tradetracker_go` (WAMP, порт 3306)
**Міграція:** version 4 (bots + bot_grids tables) — застосовано ✅

---

## Stack

| Компонент | Бібліотека | Версія |
|---|---|---|
| HTTP | `gofiber/fiber/v2` | v2.52.12 |
| DB driver | `jmoiron/sqlx` + `go-sql-driver/mysql` | latest |
| Auth | `golang-jwt/jwt/v5` | v5.3.1 |
| WebSocket | `gofiber/websocket/v2` + `fasthttp/websocket` | v2.2.1 |
| Encryption | stdlib `crypto/aes` AES-256-GCM | — |
| Migrations | `golang-migrate/migrate/v4` | v4.19.1 |
| Config | `joho/godotenv` | v1.5.1 |
| Rate limiting | `gofiber/fiber/v2/middleware/limiter` | v2.52.12 |
| Logging | stdlib `log/slog` (Go 1.21+) | — |
| Validation | `go-playground/validator/v10` | v10.30.3 |
| Redis client | `redis/go-redis/v9` | v9.20.0 |

---

## Що реалізовано (повний список)

### Пакети і файли

```
cmd/
├── server/main.go          — точка входу, DI, роути
└── migrate/main.go         — CLI для міграцій (up/down/version/force)

internal/
├── config/config.go         — .env → Config struct
├── database/db.go           — sqlx connection, pool max 25
├── middleware/auth.go       — JWT: Bearer + cookie, GetUserID()
├── models/
│   ├── user.go              — User struct (з UpdatedAt!), UserRepository
│   ├── portfolio.go         — Asset, UserPortfolio, OpenPosition,
│   │                          PositionHistory, ExternalApiCredential,
│   │                          PortfolioRepository
│   ├── order.go             — Order struct + OrderRepository (Create/GetByID/List/UpdateStatus/MarkFailed)
│   ├── smart_order.go       — SmartOrder struct + SmartOrderRepository (Create/ListActive/ListByUser/Cancel/MarkTriggered/UpdatePeak)
│   └── bot.go               — Bot + BotGrid structs + BotRepository (Create/ListRunning/UpdateGrid/AddProfit/CancelAllGrids)
├── handlers/
│   ├── auth.go              — register/login/logout/me
│   ├── portfolio.go         — CRUD credentials, comments, price update
│   ├── sync.go              — full/positions/history/prices sync endpoints
│   ├── order.go             — PlaceOrder/CancelOrder/GetOrder/ListOrders
│   ├── smart_order.go       — Create/List/Get/Cancel для умовних ордерів
│   └── bot.go               — Create/List/Get/Start/Stop/Delete для grid-ботів
├── cache/
│   ├── cache.go             — PriceStorer interface (swap memory↔Redis без змін в сервісі)
│   └── memory.go            — MemoryPriceStore (поточна реалізація)
├── scheduler/
│   └── scheduler.go         — фонові задачі з context cancellation
├── validator/
│   └── validator.go         — Validate() helper поверх go-playground/validator
├── services/
│   ├── encryption.go        — AES-256-GCM encrypt/decrypt
│   ├── sync_service.go      — паралельний синк + SyncAllUsers() для scheduler
│   ├── sync_repository.go   — всі SQL для синку (upsert, cleanup, transfer)
│   ├── price_service.go     — ціни через cache.PriceStorer + UpdateAllAssets()
│   ├── smart_order_service.go — CheckAndTrigger: перевіряє SL/TP/Trailing кожні 5с, виконує market order
│   ├── bot_service.go         — Start/Stop/CheckBots: grid bot lifecycle, counter orders кожні 10с
│   └── exchange/
│       ├── interface.go     — Exchange interface: Balance/Position/ClosedTrade + ErrNoCredentials
│       ├── registry.go      — map[name]Exchange + compile-time check (6 бірж)
│       ├── client.go        — HTTP client, retry при 429, postForm для Kraken
│       ├── helpers.go       — hmacSHA256/512, hmacSHA256Base64, sha512Hex, normalizeSymbol
│       ├── parse.go         — parseFloat, parseInt64
│       ├── trader.go        — Trader interface: PlaceOrder/CancelOrder/GetOrderStatus
│       ├── binance.go       — Binance V3 Spot + FAPI Futures
│       ├── binance_trade.go — Binance Trader (spot+futures orders)
│       ├── okx.go           — OKX V5 (Trading + Funding + Earn)
│       ├── okx_trade.go     — OKX Trader (spot+swap orders)
│       ├── bybit.go         — Bybit V5 UTA (7d windows + cursor pagination)
│       ├── bybit_trade.go   — Bybit Trader (spot+linear orders)
│       ├── gate.go          — Gate.io V4 (Spot + USDT-M Futures)
│       ├── kraken.go        — Kraken V0 (Spot + Margin, HMAC-SHA512+base64)
│       └── kucoin.go        — KuCoin V1 (Spot fills, allTickers)
└── ws/
    ├── hub.go               — thread-safe реєстр з'єднань (sync.RWMutex)
    ├── client.go            — ReadPump + WritePump goroutines, JWT auth
    ├── server.go            — broadcast loop 2s: ціни + позиції per user
    └── handler.go           — Fiber WebSocket upgrade middleware

migrations/
├── 000001_initial_schema.up.sql    — CREATE TABLE × 7 + 34 початкові активи
├── 000001_initial_schema.down.sql  — DROP TABLE у зворотньому порядку
├── 000002_orders.up.sql            — orders table (статуси, exchange_order_id)
├── 000002_orders.down.sql          — DROP TABLE orders
├── 000003_smart_orders.up.sql      — smart_orders table (SL/TP/Trailing, peak_price, callback_rate)
├── 000003_smart_orders.down.sql    — DROP TABLE smart_orders
├── 000004_bots.up.sql              — bots + bot_grids tables (grid bot lifecycle)
└── 000004_bots.down.sql            — DROP TABLE bot_grids, bots

docs/
├── getting-started.md   — встановлення, конфігурація, перший запуск
├── architecture.md      — архітектура системи, структура директорій, паттерни
├── api-reference.md     — всі HTTP та WebSocket ендпоінти
├── exchange-adapters.md — деталі інтеграції кожної біржі, auth схеми
└── trading-guide.md     — керівництво з торгівлі, lifecycle ордерів
```

### API ендпоінти

```
// Public
POST   /api/auth/register     — реєстрація, повертає JWT (cookie + body)
POST   /api/auth/login        — логін, повертає JWT (cookie + body)
POST   /api/auth/logout       — очищає cookie
GET    /health                — {"status":"ok","version":"0.1.0"}

// Protected (JWT required: Authorization: Bearer <token> або cookie)
GET    /api/auth/me

GET    /api/portfolio/                    — assets + positions + history + total_value
GET    /api/portfolio/history?limit=15&offset=0
GET    /api/portfolio/credentials
POST   /api/portfolio/credentials         — додати/оновити ключі біржі
DELETE /api/portfolio/credentials/:id
PATCH  /api/positions/:id/comment
PATCH  /api/history/:id/comment

POST   /api/sync/full                     — повний синк усіх бірж паралельно
POST   /api/sync/positions                — лише відкриті позиції
POST   /api/sync/history?days=7           — закриті угоди за N днів
GET    /api/sync/prices                   — оновити ціни активів

POST   /api/orders                        — розмістити ордер (binance/okx/bybit)
GET    /api/orders                        — список ордерів (?status=new|filled|...)
GET    /api/orders/:id                    — статус ордеру (живий запит до біржі)
DELETE /api/orders/:id                    — скасувати ордер

POST   /api/smart-orders                  — створити умовний ордер (SL/TP/Trailing)
GET    /api/smart-orders                  — список умовних ордерів користувача
GET    /api/smart-orders/:id              — один умовний ордер
DELETE /api/smart-orders/:id              — скасувати умовний ордер

POST   /api/bots                          — створити grid-бота
GET    /api/bots                          — список ботів
GET    /api/bots/:id                      — бот + рівні сітки
POST   /api/bots/:id/start                — запустити бота (виставляє ордери)
POST   /api/bots/:id/stop                 — зупинити бота (скасовує ордери)
DELETE /api/bots/:id                      — видалити зупиненого бота

GET    /ws                                — WebSocket (потребує auth повідомлення)
```

### WebSocket протокол

```json
// 1. Клієнт підключається і надсилає JWT
{ "type": "auth", "token": "eyJ..." }

// 2. Сервер підтверджує
{ "type": "auth_success", "user_id": 7 }

// 3. Сервер кожні 2 секунди надсилає
{
  "type": "update",
  "positions": [
    {
      "symbol": "BTC", "side": "LONG", "exchange": "bybit",
      "entry_price": 65000, "mark_price": 67420,
      "pnl": 241.5, "pnl_pct": 1.85, "leverage": "10x"
    }
  ],
  "spot_prices": { "BTC": 67420, "ETH": 3210, "SOL": 145 }
}
```

---

## З чого почати наступну сесію

**Фази 0–4 завершені.** Переходимо до **Фази 4 (залишок — Grid Bot) або Фази 5 (Аналітика)**.

### Фаза 5 — Аналітика ← ПОЧАТИ ЗВІДСИ

1. **portfolio_snapshots** — щоденний snapshot через scheduler, основа для PnL-графіків
2. **PnL аналітика** — winrate, середній PnL, найкращі/гірші монети
3. **Тести** — unit для exchange helpers (parseFloat, hmacSHA256), integration для OrderRepository

### Фаза 5 — Аналітика
- `portfolio_snapshots` таблиця (щоденний snapshot через scheduler)
- PnL графіки (winrate, середній PnL, найкращі монети)
- Arbitrage scanner між біржами

---

## Відомі технічні борги

| Проблема | Файл | Пріоритет |
|---|---|---|
| `getUserCreds` продубльовано в handler, smart_order_service, bot_service, ws/server | скрізь | 🟡 MEDIUM |
| `GetLivePrices(nil)` — витягує всі ціни замість потрібних | `ws/server.go` | 🟡 MEDIUM |
| Немає жодного тесту | весь проєкт | 🟡 MEDIUM |
| `gorilla/websocket` в go.mod — не використовується | `go.mod` | 🟢 LOW |
| `max()` і `roundFloat()` — треба перенести в `internal/utils` | `ws/server.go` | 🟢 LOW |

---

## Roadmap

### Фаза 2 — Production-ready backend
- [x] Cache interface (`cache.PriceStorer`) — swap memory↔Redis без змін в сервісі
- [x] Background scheduler (`scheduler.Scheduler`) — prices кожні 15с, sync кожні 5хв
- [x] Input validation (`go-playground/validator/v10`) — request structs з тегами
- [x] Redis кеш (`RedisPriceStore`) — авто-активація через `REDIS_URL` в .env
- [x] Нові біржі: Gate.io V4, Kraken V0, KuCoin V1 (всього 6 бірж)
- [ ] Тести: unit для exchange helpers, integration для DB

### Фаза 3 — Торгівля ✅ DONE
- [x] Trader interface: PlaceOrder, CancelOrder, GetOrderStatus
- [x] Binance Trader (spot + futures)
- [x] OKX Trader (spot + swap)
- [x] Bybit Trader (spot + linear)
- [x] OrderRepository + orders table (migration 000002)
- [x] OrderHandler: POST/GET/DELETE /api/orders
- [x] Order lifecycle: save→place→update / on error→mark rejected

### Фаза 4 — Smart Orders та боти (в процесі)
- [x] SmartOrder model + SmartOrderRepository (migration 000003)
- [x] SmartOrderService: CheckAndTrigger кожні 5с — SL/TP/Trailing логіка
- [x] SmartOrderHandler: POST/GET/DELETE /api/smart-orders
- [x] ErrNoCredentials sentinel error в exchange пакеті
- [x] Grid Bot: Bot + BotGrid models, BotRepository (migration 000004)
- [x] BotService: Start/Stop/CheckBots кожні 10с, counter orders після fill
- [x] BotHandler: POST/GET/Start/Stop/Delete /api/bots
- [x] Bugfix аудит: GetLivePrices(nil), roundFloat, stmt.Exec errors, nil pointer GetOrder
- [ ] DCA Bot

### Фаза 5 — Аналітика
- [ ] PnL графіки по часу (потрібна таблиця `portfolio_snapshots`)
- [ ] Winrate, середній PnL, кращі/гірші монети
- [ ] Portfolio rebalancing
- [ ] Arbitrage scanner між біржами

### Фаза 5 — Масштаб і монетизація
- [ ] Розбиття на мікросервіси (api-gateway, market-data, portfolio, trading, analytics)
- [ ] NATS/RabbitMQ message queue
- [ ] Prometheus + Grafana
- [ ] Stripe підписки (Free / Pro / Enterprise)

---

## Архітектурні принципи (не порушувати)

**1. Exchange adapters — тільки HTTP + парсинг**
Жодної бізнес-логіки. Адаптер підписує запит → парсить відповідь → повертає
нормалізований тип. Вся логіка upsert/cleanup — в `SyncService`/`SyncRepository`.

**2. Concurrency через goroutines**
Кожна біржа в синку = своя goroutine. WS клієнт = дві goroutines (read + write).
Shared state захищати через `sync.RWMutex` або канали. Не використовувати глобальні змінні.

**3. SQL — явний, без ORM**
Тільки `sqlx` + raw SQL. `INSERT IGNORE` / `ON DUPLICATE KEY UPDATE` для idempotency.
Нові таблиці або ALTER — тільки через нову міграцію в `migrations/`.

**4. Errors — завжди вгору**
Адаптери повертають `(T, error)`. Handlers логують і повертають HTTP код.
Panic тільки при старті (DB, encryption key).

**5. Фінансова точність**
БД: `DECIMAL(20,8)`. Go: `float64`. Округлення — тільки в UI layer.

---

## Команди

```bash
# Запуск сервера
go run ./cmd/server/...

# Збірка бінарника
go build -o bin/server ./cmd/server/...

# Перевірка компіляції без запуску
go build ./...

# Міграції
make migrate-up       # застосувати всі нові
make migrate-down     # відкотити останню
make migrate-version  # поточна версія

# Прибрати невикористані залежності
go mod tidy

# Тести (коли з'являться)
go test ./...
```

---

## .env (поточні значення)

```env
APP_PORT=8080
APP_DEBUG=true
JWT_SECRET=tt_go_jwt_s3cr3t_change_in_production_2025
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=tradetracker_go
APP_KEY=b84260907e594d6c97a5a8f7d98305c4   # той самий що в PHP kursova
ANTHROPIC_API_KEY=                           # заповнити коли дійдемо до AI
REDIS_URL=                                   # необов'язково: localhost:6379 для Redis кешу
```

> ⚠️ `.env` в `.gitignore`. При першому клоні — скопіювати з `.env.example`.

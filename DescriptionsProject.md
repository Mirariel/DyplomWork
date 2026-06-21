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
### Фаза 5 — Аналітика (Snapshots, TradeSummary, Coin Performance) ✅ DONE
### Фаза 6 — DCA Bot, Arbitrage Scanner, Unit Tests ✅ DONE
### Фаза 7 — Frontend (React + Vite Dashboard) ✅ DONE
### Фаза 8 — Prometheus Metrics ✅ DONE
### Фаза 9 — Telegram Push Notifications ✅ DONE
### Фаза 10 — Рефактор: getUserCreds + code splitting ✅ DONE
### Фаза 11 — Інтеграційні тести репозиторіїв ✅ DONE
### Фаза 12 — Docker + docker-compose ✅ DONE

**Сервер:** `http://localhost:8080` (Go + Fiber v2)
**Frontend (dev):** `http://localhost:5173` (Vite)
**Frontend (prod):** `http://localhost` (nginx у Docker)
**WebSocket:** `ws://localhost:8080/ws`
**БД:** MySQL `tradetracker` (локально або Docker)
**Міграції:** версія 6 (`dca_bots` table) ✅
**Docker:** `docker compose up --build -d` → все запускається автоматично (потребує запущеного Docker Desktop GUI)

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
| Rate limiting | `gofiber/fiber/v2/middleware/limiter` | built-in |
| Logging | stdlib `log/slog` (Go 1.21+) | — |
| Validation | `go-playground/validator/v10` | v10.30.3 |
| Redis client | `redis/go-redis/v9` | v9.20.0 |
| Metrics | `prometheus/client_golang` + `promauto` | latest |

**Frontend stack:**

| Компонент | Бібліотека | Версія |
|---|---|---|
| Build tool | `vite` + `@vitejs/plugin-react` | v6.4.3 / v4.5.2 |
| UI framework | `react` + `react-dom` | v18.3.1 |
| Routing | `react-router-dom` | v6.30.0 |
| Data fetching | `@tanstack/react-query` | v5.80.6 |
| HTTP client | `axios` | v1.9.0 |
| Charts | `recharts` | v2.15.4 |
| Icons | `lucide-react` | v0.511.0 |
| CSS | `tailwindcss` + `@tailwindcss/vite` | v4.1.8 |
| Language | TypeScript (strict) | v5.8.3 |

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
│   ├── user.go              — User struct, UserRepository
│   ├── portfolio.go         — Asset, UserPortfolio, OpenPosition,
│   │                          PositionHistory, ExternalApiCredential,
│   │                          PortfolioRepository
│   ├── order.go             — Order struct + OrderRepository
│   ├── smart_order.go       — SmartOrder struct + SmartOrderRepository
│   ├── bot.go               — Bot + BotGrid structs + BotRepository
│   ├── snapshot.go          — PortfolioSnapshot struct + SnapshotRepository
│   ├── dca_bot.go           — DCABot struct + DCABotRepository
│   ├── testmain_test.go     — TestMain: авто-створення tradetracker_test DB, migrate UP
│   ├── user_repo_test.go    — інтеграційні тести UserRepository (4 тести)
│   ├── order_repo_test.go   — інтеграційні тести OrderRepository (5 тестів)
│   ├── smart_order_repo_test.go — інтеграційні тести SmartOrderRepository (7 тестів)
│   ├── bot_repo_test.go     — інтеграційні тести BotRepository (6 тестів)
│   ├── dca_bot_repo_test.go — інтеграційні тести DCABotRepository (6 тестів)
│   └── snapshot_repo_test.go — інтеграційні тести SnapshotRepository (4 тести)
├── handlers/
│   ├── auth.go              — register/login/logout/me
│   ├── portfolio.go         — CRUD credentials, comments
│   ├── sync.go              — full/positions/history/prices sync endpoints
│   ├── order.go             — PlaceOrder/CancelOrder/GetOrder/ListOrders
│   ├── smart_order.go       — Create/List/Get/Cancel
│   ├── bot.go               — Create/List/Get/Start/Stop/Delete
│   ├── analytics.go         — Summary/Coins/Snapshots/TakeSnapshot/Arbitrage
│   └── dca.go               — Create/List/Get/Start/Stop/Delete
├── cache/
│   ├── cache.go             — PriceStorer interface
│   ├── memory.go            — MemoryPriceStore (in-memory TTL cache)
│   └── redis.go             — RedisPriceStore (авто-активація через REDIS_URL)
├── scheduler/
│   └── scheduler.go         — фонові задачі з context cancellation + Prometheus metrics
├── metrics/
│   └── metrics.go           — HTTP counter/histogram, WS gauge, scheduler histogram
├── notify/
│   └── telegram.go          — Telegram Bot API (no-op якщо токен не заданий)
├── validator/
│   └── validator.go         — Validate() helper
├── services/
│   ├── creds.go             — GetUserCreds() shared helper (розшифрування API credentials)
│   ├── encryption.go        — AES-256-GCM encrypt/decrypt
│   ├── sync_service.go      — паралельний синк + SyncAllUsers()
│   ├── sync_repository.go   — SQL для синку (upsert, cleanup, transfer)
│   ├── price_service.go     — UpdateAllAssets() через cache.PriceStorer
│   ├── smart_order_service.go — CheckAndTrigger кожні 5с (SL/TP/Trailing)
│   ├── bot_service.go         — Start/Stop/CheckBots кожні 10с
│   ├── analytics_service.go   — TakeSnapshot, GetTradeSummary, GetCoinPerformance, GetArbitrage
│   ├── dca_service.go         — Start/Stop/CheckAndBuy кожні 5хв
│   └── exchange/
│       ├── interface.go + registry.go + client.go + helpers.go + parse.go + trader.go
│       ├── binance.go + binance_trade.go   — Binance V3 Spot + FAPI Futures
│       ├── okx.go + okx_trade.go           — OKX V5 Spot + Swap
│       ├── bybit.go + bybit_trade.go       — Bybit V5 UTA Spot + Linear
│       ├── gate.go / kraken.go / kucoin.go — Gate.io V4, Kraken V0, KuCoin V1
│       ├── helpers_test.go  — unit тести: hmac, sha512, normalizeSymbol
│       └── parse_test.go    — unit тести: parseFloat, parseInt64
└── ws/
    ├── hub.go / client.go / server.go / handler.go
    └── server_test.go       — unit тести: roundFloat, formatLeverage

frontend/
├── package.json / vite.config.ts / tsconfig.json
├── Dockerfile               — node:20-alpine → nginx:1.27-alpine
├── nginx.conf               — SPA routing, /api proxy, /ws WebSocket, asset caching
├── .dockerignore
└── src/
    ├── App.tsx              — React.lazy для всіх 9 сторінок + Suspense
    ├── api.ts               — 30+ типізованих API функцій
    ├── ws.ts                — useWebSocket hook (auto-reconnect 3s)
    ├── context/AuthContext.tsx
    ├── components/Layout.tsx
    └── pages/ (Login, Register, Dashboard, Portfolio, Orders,
                SmartOrders, GridBots, DCABots, Analytics)

migrations/ (000001–000006)

Dockerfile                   — golang:1.26-alpine → alpine:3.20
.dockerignore
docker-compose.yml           — db, redis, migrate (one-shot), api, frontend
```

### API ендпоінти

```
POST   /api/auth/register | login | logout
GET    /api/auth/me
GET    /health
GET    /metrics  (Prometheus)

GET    /api/portfolio/ | /history | /credentials
POST   /api/portfolio/credentials
DELETE /api/portfolio/credentials/:id
PATCH  /api/positions/:id/comment | /api/history/:id/comment

POST   /api/sync/full | /positions | /history
GET    /api/sync/prices

POST   /api/orders              GET    /api/orders
GET    /api/orders/:id          DELETE /api/orders/:id

POST   /api/smart-orders        GET    /api/smart-orders
GET    /api/smart-orders/:id    DELETE /api/smart-orders/:id

POST   /api/bots                GET    /api/bots
GET    /api/bots/:id            DELETE /api/bots/:id
POST   /api/bots/:id/start | /stop

GET    /api/analytics/summary | /coins | /snapshots?days=30 | /arbitrage?min_spread=0.5
POST   /api/analytics/snapshot

POST   /api/dca                 GET    /api/dca
GET    /api/dca/:id             DELETE /api/dca/:id
POST   /api/dca/:id/start?buy_now=true | /stop

GET    /ws   (WebSocket)
```

### WebSocket протокол

```json
// Клієнт → { "type": "auth", "token": "eyJ..." }
// Сервер → { "type": "auth_success", "user_id": 7 }
// Сервер кожні 2с →
{
  "type": "update",
  "positions": [{ "symbol": "BTC", "pnl": 241.5, "pnl_pct": 1.85, ... }],
  "spot_prices": { "BTC": 67420, "ETH": 3210 }
}
```

---

## Технічні борги

Усі відомі технічні борги вирішені.

| Проблема | Статус |
|---|---|
| `getUserCreds` дублювання в 4 місцях | ✅ `services.GetUserCreds()` в `creds.go` |
| Немає інтеграційних тестів | ✅ 33 тест-кейси в `internal/models/` |
| Frontend bundle 725 kB | ✅ Code splitting, сторінки 3–11 kB кожна |
| `gorilla/websocket` в go.mod | ✅ Видалено через `go mod tidy` |

---

## З чого почати наступну сесію

**Фази 0–12 завершені.** Проєкт повністю функціональний, технічних боргів немає.

Можливі напрямки:
- **E2E тести** — handlers через `net/http/httptest`
- **Grafana** — dashboard для Prometheus метрик (окремий сервіс у docker-compose)
- **Мікросервіси** — розбиття api-gateway / market-data / trading / analytics

---

## Roadmap

### Фаза 0 — Foundation ✅ DONE
- [x] Go модуль, `.env`, Makefile, sqlx, MySQL driver
- [x] `config.Load()`, `database.Connect()`, connection pool

### Фаза 1 — Перший живий запуск ✅ DONE
- [x] 7 таблиць + 34 seed активи (migration 000001)
- [x] UserRepository, PortfolioRepository
- [x] JWT auth (register/login/logout/me)
- [x] Rate limiting, slog, graceful shutdown 5s

### Фаза 2 — Production-ready backend ✅ DONE
- [x] `cache.PriceStorer` interface — swap memory↔Redis без змін в сервісі
- [x] Background scheduler з context cancellation
- [x] Input validation (`go-playground/validator/v10`)
- [x] Redis кеш — авто-активація через `REDIS_URL`
- [x] 6 бірж: Binance, OKX, Bybit, Gate.io, Kraken, KuCoin

### Фаза 3 — Торгівля ✅ DONE
- [x] Trader interface: PlaceOrder, CancelOrder, GetOrderStatus
- [x] Binance, OKX, Bybit Trader (spot + futures/swap)
- [x] OrderRepository + orders table (migration 000002)
- [x] OrderHandler: POST/GET/DELETE /api/orders
- [x] Order lifecycle: save → place → update / on error → rejected

### Фаза 4 — Smart Orders + Grid Bot ✅ DONE
- [x] SmartOrder model + SmartOrderRepository (migration 000003)
- [x] SmartOrderService: CheckAndTrigger кожні 5с — SL/TP/Trailing
- [x] SmartOrderHandler: POST/GET/DELETE /api/smart-orders
- [x] ErrNoCredentials sentinel error
- [x] Grid Bot: Bot + BotGrid models (migration 000004)
- [x] BotService: Start/Stop/CheckBots кожні 10с, counter orders після fill
- [x] BotHandler: POST/GET/Start/Stop/Delete /api/bots
- [x] Bugfix аудит: GetLivePrices(nil), roundFloat, stmt.Exec errors, nil pointer

### Фаза 5 — Аналітика ✅ DONE
- [x] portfolio_snapshots (migration 000005), UPSERT щогодини
- [x] AnalyticsService: TakeSnapshot, GetTradeSummary, GetCoinPerformance
- [x] AnalyticsHandler: /summary, /coins, /snapshots, /snapshot
- [x] TradeSummary: winrate, avg_pnl, profit_factor, best/worst trade

### Фаза 6 — DCA Bot, Arbitrage Scanner, Unit Tests ✅ DONE
- [x] DCABot model + DCABotRepository (migration 000006)
- [x] DCAService: Start/Stop/CheckAndBuy кожні 5хв (qty = amount_usd / price)
- [x] DCAHandler: POST/GET/Start/Stop/Delete /api/dca + ?buy_now=true
- [x] ArbitrageScanner: GET /api/analytics/arbitrage?min_spread=0.5
- [x] Unit tests: exchange helpers, parse, ws server (33/33 PASS)

### Фаза 7 — Frontend ✅ DONE
- [x] React 18 + Vite 6 + TypeScript + Tailwind CSS v4
- [x] React Query v5, Axios, Recharts, Lucide
- [x] 9 сторінок: Dashboard, Portfolio, Orders, SmartOrders, GridBots, DCABots, Analytics, Login, Register
- [x] WebSocket hook (auto-reconnect, live prices + positions)
- [x] Dark sidebar layout, auth context (JWT localStorage)
- [x] `vite-env.d.ts` для CSS module типів

### Фаза 8 — Prometheus Metrics ✅ DONE
- [x] HTTP request counter/histogram (per method+route+status)
- [x] WebSocket clients gauge, active bots/smart-orders gauges
- [x] Scheduler job duration histogram
- [x] GET /metrics (promhttp через gofiber/adaptor)

### Фаза 9 — Telegram Push Notifications ✅ DONE
- [x] `notify.Notifier` — no-op якщо токен не заданий
- [x] SmartOrderTriggered/Failed, DCABought, GridBotError
- [x] Async goroutine (не блокує scheduler)

### Фаза 10 — Рефактор + Code Splitting ✅ DONE
- [x] `services.GetUserCreds()` — shared helper замість дублювання в 4 місцях
- [x] React.lazy для всіх 9 сторінок + Suspense fallback spinner
- [x] Vite `manualChunks`: vendor-react, vendor-query, vendor-charts, vendor-icons, vendor-axios
- [x] Результат: сторінки 3–11 kB (було один bundle ~725 kB)

### Фаза 11 — Інтеграційні тести репозиторіїв ✅ DONE
- [x] TestMain: авто-створення `tradetracker_test` DB + golang-migrate UP
- [x] `truncateAll()` з FK_CHECKS=0 перед кожним тестом
- [x] 33 тест-кейси: UserRepository, OrderRepository, SmartOrderRepository,
      BotRepository (grid lifecycle), DCABotRepository (ListDue), SnapshotRepository (UPSERT)
- [x] Запуск: `go test ./internal/models/...` (потребує MySQL)

### Фаза 12 — Docker + docker-compose ✅ DONE
- [x] `Dockerfile`: golang:1.26-alpine → alpine:3.20 (server + migrate бінарники)
- [x] `frontend/Dockerfile`: node:20-alpine → nginx:1.27-alpine
- [x] `frontend/nginx.conf`: SPA routing, /api proxy, /ws WebSocket upgrade, asset caching
- [x] `docker-compose.yml`: db (MySQL 8), redis, migrate (one-shot), api, frontend
- [x] Залежності: migrate чекає DB healthcheck, api чекає migrate completion
- [x] Запуск однією командою: `docker compose up --build -d`
- [x] Docker Desktop 4.78.0 встановлено (winget); daemon стартує через GUI (не CLI)

### Фаза 13 — Майбутнє (не реалізовано)
- [ ] E2E тести handlers через `net/http/httptest`
- [ ] Grafana dashboard для Prometheus метрик
- [ ] Розбиття на мікросервіси (api-gateway, market-data, trading, analytics)
- [ ] NATS/RabbitMQ message queue між сервісами
- [ ] Stripe підписки (Free / Pro / Enterprise)

---

## Архітектурні принципи (не порушувати)

**1. Exchange adapters — тільки HTTP + парсинг**
Жодної бізнес-логіки. Адаптер підписує запит → парсить відповідь → повертає
нормалізований тип. Вся логіка upsert/cleanup — в `SyncService`/`SyncRepository`.

**2. Concurrency через goroutines**
Кожна біржа в синку = своя goroutine. WS клієнт = дві goroutines (read + write).
Shared state захищати через `sync.RWMutex` або канали.

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
# Локальний запуск
go run ./cmd/server/...
go build -o bin/server ./cmd/server/...
go build ./...

# Тести
go test ./...                            # всі (unit + integration)
go test ./internal/models/...            # лише інтеграційні (потрібна MySQL)
go test ./internal/services/exchange/... # лише unit

# Міграції (локально)
make migrate-up
make migrate-down
make migrate-version

# Docker
make docker-up       # зібрати і запустити всі сервіси у фоні
make docker-down     # зупинити
make docker-logs     # tail логів
make docker-reset    # зупинити + видалити volumes (скидає БД)

# Frontend (dev)
cd frontend && npm run dev
cd frontend && npm run build

go mod tidy          # прибрати невикористані залежності
```

---

## Змінні середовища (.env.example)

```env
APP_PORT=8080
APP_DEBUG=true
JWT_SECRET=your-super-secret-jwt-key-change-in-production

DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=tradetracker

# 32 символи рівно — AES-256-GCM
APP_KEY=your-32-character-encryption-key!

# Redis (необов'язково — без нього in-memory cache)
REDIS_URL=

# Telegram сповіщення (необов'язково)
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
```

> ⚠️ `.env` в `.gitignore`. При першому клоні — `cp .env.example .env` і заповнити секрети.

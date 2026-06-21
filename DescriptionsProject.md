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
### Фаза 2 — Production-ready backend ✅ DONE
### Фаза 3 — Торгівля (Binance + OKX + Bybit PlaceOrder/Cancel/Status) ✅ DONE
### Фаза 4 — Smart Orders + Grid Bot ✅ DONE
### Фаза 5 — Аналітика (Snapshots, TradeSummary, Coin Performance) ✅ DONE
### Фаза 6 — DCA Bot, Arbitrage Scanner, Unit Tests ✅ DONE
### Фаза 7 — Frontend (React + Vite Dashboard) ✅ DONE
### Фаза 8 — Prometheus Metrics ✅ DONE
### Фаза 9 — Telegram Push Notifications ✅ DONE
### Фаза 10 — Рефактор: getUserCreds + code splitting ✅ DONE
### Фаза 11 — Інтеграційні тести репозиторіїв ✅ DONE
### Фаза 12 — Docker + docker-compose ✅ DONE
### Фаза 13 — Grafana monitoring dashboard ✅ DONE
### Фаза 14 — Profile Settings сторінка + user API ✅ DONE
### Фаза 15 — Мікросервіси (api-gateway / market-data / trading / analytics + NATS) ✅ DONE

**Сервіси:**

| Сервіс | URL | Призначення |
|---|---|---|
| Frontend (React/nginx) | http://localhost:3000 | SPA |
| API Gateway (Go/Fiber) | http://localhost:8080 | JWT, WS, proxy |
| market-data | http://localhost:8081 | ціни, синк |
| trading | http://localhost:8082 | ордери, боти |
| analytics | http://localhost:8083 | статистика |
| NATS | localhost:4222 / :8222 | повідомлення |
| Prometheus | http://localhost:9090 | метрики |
| Grafana | http://localhost:3001 | dashboard (admin/admin) |
| MySQL | localhost:3306 | |
| Redis | localhost:6379 | price cache |

**БД:** `tradetracker_go` (MySQL), версія міграції: 6
**Запуск:** `docker compose up --build -d` → 10 контейнерів

---

## Stack

### Backend

| Компонент | Бібліотека | Версія |
|---|---|---|
| HTTP | `gofiber/fiber/v2` | v2.52.12 |
| DB driver | `jmoiron/sqlx` + `go-sql-driver/mysql` | latest |
| Auth | `golang-jwt/jwt/v5` | v5.3.1 |
| WebSocket | `gofiber/websocket/v2` | v2.2.1 |
| Encryption | stdlib `crypto/aes` AES-256-GCM | — |
| Migrations | `golang-migrate/migrate/v4` | v4.19.1 |
| Config | `joho/godotenv` | v1.5.1 |
| Rate limiting | `gofiber/fiber/v2/middleware/limiter` | built-in |
| Logging | stdlib `log/slog` (Go 1.21+) | — |
| Validation | `go-playground/validator/v10` | v10.30.3 |
| Redis client | `redis/go-redis/v9` | v9.20.0 |
| Metrics | `prometheus/client_golang` + `promauto` | v1.23.2 |
| Message bus | `nats-io/nats.go` | v1.52.0 |

### Frontend

| Компонент | Бібліотека | Версія |
|---|---|---|
| Build tool | `vite` + `@vitejs/plugin-react` | v6.4.3 |
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
tradetracker-go/
├── cmd/
│   ├── api-gateway/main.go     — JWT, rate limit, WS hub, HTTP reverse-proxy, auth/profile handlers
│   ├── market-data/main.go     — sync endpoints, price scheduler (30 s → Redis + NATS publish)
│   ├── trading/main.go         — portfolio/orders/bots/DCA handlers + schedulers + NATS subscribe
│   ├── analytics/main.go       — analytics endpoints (summary/coins/snapshots/arbitrage)
│   ├── server/main.go          — монолітна точка входу (legacy reference, не в Docker)
│   └── migrate/main.go         — CLI для міграцій (up/down/version/force)
│
├── internal/
│   ├── config/config.go        — Config struct (NatsURL, MarketDataURL, TradingURL, AnalyticsURL)
│   ├── database/db.go          — sqlx connection, pool max 25
│   ├── middleware/
│   │   ├── auth.go             — JWTAuth(): Bearer + cookie; GetUserID()
│   │   └── internal.go         — InternalAuth(): X-Internal-User-ID header → Locals
│   ├── nats/
│   │   └── bus.go              — Bus (Connect/Publish/Subscribe/Close), SubjPricesUpdated, PricesMsg
│   ├── models/
│   │   ├── user.go             — User + UserRepository (Create/Find/UpdateProfile/UpdatePassword)
│   │   ├── portfolio.go        — Asset, Position, History, Credentials + PortfolioRepository
│   │   ├── order.go            — Order + OrderRepository
│   │   ├── smart_order.go      — SmartOrder + SmartOrderRepository
│   │   ├── bot.go              — Bot + BotGrid + BotRepository
│   │   ├── snapshot.go         — PortfolioSnapshot + SnapshotRepository
│   │   ├── dca_bot.go          — DCABot + DCABotRepository
│   │   └── *_test.go           — 33 інтеграційні тести
│   ├── handlers/
│   │   ├── auth.go             — Register/Login/Logout/Me/UpdateProfile/ChangePassword
│   │   ├── portfolio.go        — CRUD credentials, comments
│   │   ├── sync.go             — full/positions/history/prices
│   │   ├── order.go            — PlaceOrder/Cancel/Get/List
│   │   ├── smart_order.go      — Create/List/Get/Cancel
│   │   ├── bot.go              — Create/List/Get/Start/Stop/Delete
│   │   ├── analytics.go        — Summary/Coins/Snapshots/TakeSnapshot/Arbitrage
│   │   └── dca.go              — Create/List/Get/Start/Stop/Delete
│   ├── services/
│   │   ├── creds.go            — GetUserCreds() shared helper
│   │   ├── encryption.go       — AES-256-GCM encrypt/decrypt
│   │   ├── sync_service.go     — SyncUser/SyncAllUsers (паралельно per exchange)
│   │   ├── sync_repository.go  — SQL для синку (upsert, cleanup)
│   │   ├── price_service.go    — UpdateAllAssets() + GetLivePrices() через PriceStorer
│   │   ├── smart_order_service.go — CheckAndTrigger: SL/TP/Trailing
│   │   ├── bot_service.go      — Start/Stop/CheckBots (10 s)
│   │   ├── analytics_service.go — TakeSnapshot, TradeSummary, CoinPerf, Arbitrage
│   │   ├── dca_service.go      — Start/Stop/CheckAndBuy (5 min)
│   │   └── exchange/           — 6 бірж: Binance/OKX/Bybit/Gate/Kraken/KuCoin
│   │       ├── interface.go + registry.go + client.go + helpers.go + parse.go + trader.go
│   │       ├── binance.go + binance_trade.go
│   │       ├── okx.go + okx_trade.go
│   │       ├── bybit.go + bybit_trade.go
│   │       ├── gate.go / kraken.go / kucoin.go
│   │       ├── helpers_test.go — unit тести hmac/sha512/normalizeSymbol
│   │       └── parse_test.go   — unit тести parseFloat/parseInt64
│   ├── cache/
│   │   ├── cache.go            — PriceStorer interface
│   │   ├── memory.go           — MemoryPriceStore (TTL 30 s, sync.RWMutex)
│   │   └── redis.go            — RedisPriceStore (shared між api-gateway і market-data)
│   ├── scheduler/
│   │   └── scheduler.go        — background jobs з context cancellation + Prometheus
│   ├── metrics/
│   │   └── metrics.go          — HTTP counter/histogram, WS/bots/smart-orders gauges
│   ├── notify/
│   │   └── telegram.go         — Telegram Bot API (no-op без токена)
│   ├── validator/
│   │   └── validator.go        — Validate() helper
│   └── ws/
│       ├── hub.go              — реєстр WS-з'єднань (sync.RWMutex)
│       ├── client.go           — ReadPump + WritePump goroutines
│       ├── server.go           — broadcast loop (2 s): positions + spot prices
│       ├── handler.go          — Fiber WS upgrade
│       └── server_test.go      — unit тести roundFloat/formatLeverage
│
├── migrations/                 — SQL файли 000001–000006
│   ├── 000001_initial_schema   — users, assets, portfolios, positions, history, credentials
│   ├── 000002_orders           — orders table
│   ├── 000003_smart_orders     — smart_orders table
│   ├── 000004_bots             — bots + bot_grids tables
│   ├── 000005_snapshots        — portfolio_snapshots table
│   └── 000006_dca_bots         — dca_bots table
│
├── frontend/
│   ├── Dockerfile              — node:20-alpine → nginx:1.27-alpine
│   ├── nginx.conf              — SPA routing, /api proxy → :8080, /ws WS upgrade, asset caching
│   ├── vite.config.ts          — manualChunks: vendor-react/query/charts/icons/axios
│   └── src/
│       ├── App.tsx             — React.lazy (10 сторінок) + Suspense + route guards
│       ├── api.ts              — 35+ типізованих API функцій
│       ├── ws.ts               — useWebSocket hook (auto-reconnect 3 s)
│       ├── context/AuthContext.tsx — user/token/login/logout/updateUser
│       ├── components/Layout.tsx   — sidebar (8 nav items + Settings), user footer
│       └── pages/              — Login, Register, Dashboard, Portfolio, Orders,
│                                  SmartOrders, GridBots, DCABots, Analytics, Settings
│
├── monitoring/
│   ├── prometheus.yml          — 4 scrape targets (api-gateway, market-data, trading, analytics)
│   └── grafana/
│       ├── provisioning/       — auto-provisioning datasource + dashboard provider
│       └── dashboards/tradetracker.json — 10 panels: HTTP rate/latency/errors, WS, bots, scheduler
│
├── Dockerfile                  — golang:1.26-alpine → alpine:3.20 (5 бінарників в одному image)
├── docker-compose.yml          — 10 сервісів: db, redis, nats, migrate, 4×Go, frontend, prometheus, grafana
└── docs/                       — ця документація
```

### API ендпоінти (через api-gateway)

```
POST   /api/auth/register | login | logout
GET    /api/auth/me
PATCH  /api/user/profile | /api/user/password

GET    /health  (per service)
GET    /metrics (per service, Prometheus)

GET    /api/portfolio/ | /history | /credentials
POST   /api/portfolio/credentials
DELETE /api/portfolio/credentials/:id
PATCH  /api/positions/:id/comment | /api/history/:id/comment

POST   /api/sync/full | /positions | /history
GET    /api/sync/prices

POST/GET/DELETE  /api/orders | /api/orders/:id

POST/GET/DELETE  /api/smart-orders | /api/smart-orders/:id

POST/GET/DELETE  /api/bots | /api/bots/:id
POST             /api/bots/:id/start | /stop

GET    /api/analytics/summary | /coins | /snapshots?days=30 | /arbitrage?min_spread=0.5
POST   /api/analytics/snapshot

POST/GET/DELETE  /api/dca | /api/dca/:id
POST             /api/dca/:id/start?buy_now=true | /stop

GET    /ws   (WebSocket)
```

### WebSocket протокол

```json
// Клієнт → { "type": "auth", "token": "eyJ..." }
// Сервер → { "type": "auth_success", "user_id": 7 }
// Сервер кожні 2 с →
{
  "type": "update",
  "positions": [{ "symbol": "BTC", "pnl": 241.5, "pnl_pct": 1.85, ... }],
  "spot_prices": { "BTC": 67420, "ETH": 3210 }
}
```

### Monitoring

- **Prometheus** — 4 scrape targets (api-gateway, market-data, trading, analytics)
- **Grafana** — TradeTracker dashboard: HTTP rate, latency P50/P95/P99, error rate,
  WS clients, active bots, smart orders, scheduler job duration/errors
- **NATS monitoring** — `http://localhost:8222`

---

## Технічні борги

Усі відомі технічні борги вирішені.

| Проблема | Статус |
|---|---|
| `getUserCreds` дублювання в 4 місцях | ✅ `services.GetUserCreds()` |
| Немає інтеграційних тестів | ✅ 33 тест-кейси в `internal/models/` |
| Frontend bundle 725 kB | ✅ Code splitting |
| `gorilla/websocket` в go.mod | ✅ Видалено |
| Монолітна архітектура | ✅ 4 мікросервіси + NATS |

---

## З чого почати наступну сесію

**Фази 0–15 завершені.** Проєкт повністю функціональний.

Можливі напрямки:
- **E2E тести** — handlers через `net/http/httptest` або `testcontainers`
- **Окрема БД на сервіс** — повна ізоляція (окремі MySQL схеми/інстанси)
- **Service discovery** — замість хардкоду URL (Consul або env-based)
- **gRPC** — замість HTTP proxy для внутрішньої комунікації між сервісами
- **Stripe підписки** — Free / Pro / Enterprise tier

---

## Roadmap

### Фаза 0 — Foundation ✅
- [x] Go модуль, `.env`, sqlx, MySQL driver, `config.Load()`, connection pool

### Фаза 1 — Перший живий запуск ✅
- [x] 7 таблиць + 34 seed активи, UserRepository, JWT auth, rate limiting, graceful shutdown

### Фаза 2 — Production-ready backend ✅
- [x] `cache.PriceStorer`, background scheduler, input validation, Redis, 6 бірж

### Фаза 3 — Торгівля ✅
- [x] Trader interface (PlaceOrder/Cancel/Status), Binance/OKX/Bybit, OrderHandler

### Фаза 4 — Smart Orders + Grid Bot ✅
- [x] SmartOrderService CheckAndTrigger (SL/TP/Trailing), BotService Start/Stop/CheckBots

### Фаза 5 — Аналітика ✅
- [x] portfolio_snapshots, TradeSummary (winrate, profit_factor), CoinPerformance

### Фаза 6 — DCA Bot, Arbitrage, Unit Tests ✅
- [x] DCAService, ArbitrageScanner, 33 unit/integration тести

### Фаза 7 — Frontend ✅
- [x] React 18 + Vite + TypeScript + Tailwind, 9 сторінок, WS hook

### Фаза 8 — Prometheus Metrics ✅
- [x] HTTP counter/histogram, WS gauge, scheduler histogram, /metrics endpoint

### Фаза 9 — Telegram Notifications ✅
- [x] notify.Notifier (no-op без токена), async goroutine

### Фаза 10 — Рефактор + Code Splitting ✅
- [x] `services.GetUserCreds()`, React.lazy, Vite manualChunks

### Фаза 11 — Інтеграційні тести ✅
- [x] TestMain (авто-DB), truncateAll, 33 тест-кейси у models/

### Фаза 12 — Docker + docker-compose ✅
- [x] Dockerfile (golang:1.26-alpine → alpine:3.20), frontend nginx, compose

### Фаза 13 — Grafana Monitoring ✅
- [x] Grafana 11.0.0, auto-provisioning datasource + dashboard, 10 panels
- [x] Prometheus 2.53.0, prometheus.yml scrape config

### Фаза 14 — Profile Settings ✅
- [x] PATCH /api/user/profile (username, email)
- [x] PATCH /api/user/password (current + new password, bcrypt)
- [x] Settings сторінка (Account Info + Profile form + Password form)
- [x] Sidebar: Settings nav item, username у footer, avatar link

### Фаза 15 — Мікросервіси + NATS ✅
- [x] 4 сервіси: api-gateway (8080), market-data (8081), trading (8082), analytics (8083)
- [x] `internal/nats/bus.go` — NATS клієнт + `market.prices.updated` subject + PricesMsg type
- [x] `internal/middleware/internal.go` — InternalAuth (X-Internal-User-ID header)
- [x] HTTP reverse-proxy в api-gateway (виставляє X-Internal-User-ID після JWT validation)
- [x] NATS pub/sub: market-data публікує ціни → trading тригерить smart-order checks
- [x] Єдиний Dockerfile (5 бінарників), CMD override per service у docker-compose
- [x] docker-compose.yml: 10 сервісів (db, redis, nats, migrate, 4×Go, frontend, prometheus, grafana)
- [x] Prometheus: 4 scrape targets замість 1

### Фаза 16 — Майбутнє
- [ ] E2E тести (httptest / testcontainers)
- [ ] Окрема БД на сервіс (повна ізоляція)
- [ ] gRPC замість HTTP proxy для внутрішньої комунікації
- [ ] Service discovery (Consul або env-based)
- [ ] Stripe підписки (Free / Pro / Enterprise)

---

## Архітектурні принципи (не порушувати)

**1. Exchange adapters — тільки HTTP + парсинг**
Жодної бізнес-логіки. Адаптер підписує → парсить → повертає нормалізований тип.

**2. Concurrency через goroutines**
Кожна біржа в синку = goroutine. WS клієнт = 2 goroutines. Scheduler job = goroutine.
Shared state — `sync.RWMutex` або канали.

**3. SQL — явний, без ORM**
`sqlx` + raw SQL. `INSERT IGNORE` / `ON DUPLICATE KEY UPDATE`. Нові таблиці — тільки через міграцію.

**4. Errors — завжди вгору**
Handlers логують і повертають HTTP код. Panic тільки при старті.

**5. Фінансова точність**
БД: `DECIMAL(20,8)`. Go: `float64`. Округлення — тільки в UI layer.

**6. Мікросервіси — через gateway**
Ніяких прямих викликів між сервісами крім NATS events. Клієнти знають тільки про api-gateway.
Downstream сервіси довіряють `X-Internal-User-ID` (не валідують JWT).

---

## Команди

```bash
# Локальний запуск (монолітний режим, legacy)
go run ./cmd/server/...

# Локальний запуск окремих сервісів
APP_PORT=8080 go run ./cmd/api-gateway/...
APP_PORT=8081 go run ./cmd/market-data/...
APP_PORT=8082 go run ./cmd/trading/...
APP_PORT=8083 go run ./cmd/analytics/...

# Збірка всіх бінарників
go build ./cmd/api-gateway ./cmd/market-data ./cmd/trading ./cmd/analytics ./cmd/migrate

# Тести
go test ./...                            # всі
go test ./internal/models/...            # інтеграційні (потрібна MySQL)
go test ./internal/services/exchange/... # unit

# Docker (рекомендовано)
docker compose up --build -d    # зібрати і запустити 10 сервісів
docker compose ps               # статус
docker compose logs -f trading  # логи конкретного сервісу
docker compose down             # зупинити

# Міграції
go run ./cmd/migrate -cmd up
go run ./cmd/migrate -cmd version

# Frontend (dev)
cd frontend && npm run dev   # → http://localhost:5173

go mod tidy  # прибрати невикористані залежності
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
DB_NAME=tradetracker_go

# 32 символи рівно — AES-256-GCM
APP_KEY=your-32-character-encryption-key!

# Redis (необов'язково — без нього in-memory cache)
REDIS_URL=

# NATS (необов'язково — без нього smart orders на fallback timer)
NATS_URL=nats://localhost:4222

# Внутрішні URL сервісів (для api-gateway)
MARKET_DATA_URL=http://localhost:8081
TRADING_URL=http://localhost:8082
ANALYTICS_URL=http://localhost:8083

# Telegram сповіщення (необов'язково)
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
```

> `.env` в `.gitignore`. При першому клоні — `cp .env.example .env` і заповнити секрети.

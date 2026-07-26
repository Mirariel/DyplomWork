# TradeTracker Go — CLAUDE.md

Цей файл є живою документацією проєкту для Claude Code.
Оновлювати після кожної значимої зміни архітектури або завершення фази.

---

## Контекст і ціль

**Що це:** Переписування PHP-прототипу (`C:\wamp64\www\kursova`) на Go з метою перетворити
його на повноцінний трейдинг-продукт рівня [Bitsgap](https://bitsgap.com) —
мультибіржовий портфельний трекер з торговими ботами, smart orders та аналітикою.

**Чому Go:** Криптотрейдин — це latency-sensitive домен. PHP з синхронним виконанням
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
### Фаза 16.1 — Futures Positions + Auto-Discovery ✅ DONE
### Фаза 16.2 — Spot Trades History ✅ DONE
### Фаза 16.3 — AI Advisor (Claude API) ✅ DONE
### Фаза 16.4 — Portfolio Overhaul (Balances + recharts PriceChart) ✅ DONE
### Фаза 16.5 — Dashboard redesign (Exchange Filter + Live Prices Grid + Top Symbols) ✅ DONE
### Фаза 16.6 — Analytics Scheduler + PnL% formula fix ✅ DONE
### Фаза 16.7 — History Data Quality Fix (Leverage + Margin + ctVal) ✅ DONE
### Фаза 17 — Unified Orders + Advanced Order System ✅ DONE

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

**БД:** `tradetracker_go` (MySQL), версія міграції: 14
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
| Candlestick charts | `lightweight-charts` | latest |
| Form validation | `zod` + `@hookform/resolvers` | latest |
| Forms | `react-hook-form` | latest |
| Testing | `vitest` | latest |
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
│   │   ├── portfolio.go        — Asset, Position, History, ExternalApiCredential (label, api_key_hint) + PortfolioRepository
│   │   ├── order.go            — Order + OrderRepository (credential_id, leverage)
│   │   ├── smart_order.go      — SmartOrder + SmartOrderRepository (credential_id, leverage)
│   │   ├── credential_group.go — CredentialGroup + CredentialGroupMember + CredentialGroupRepository
│   │   ├── bot.go              — Bot + BotGrid + BotRepository
│   │   ├── snapshot.go         — PortfolioSnapshot + SnapshotRepository
│   │   ├── dca_bot.go          — DCABot + DCABotRepository
│   │   ├── futures_position.go — FuturesPosition + FuturesPositionRepository (Upsert/GetByUser/DeleteStale)
│   │   ├── spot_trade.go       — SpotTrade + SpotTradeRepository (Upsert/GetByUser)
│   │   └── *_test.go           — 33 інтеграційні тести
│   ├── handlers/
│   │   ├── auth.go             — Register/Login/Logout/Me/UpdateProfile/ChangePassword
│   │   ├── portfolio.go        — CRUD credentials (label, api_key_hint), comments, SetCredentialHook
│   │   ├── sync.go             — full/positions/history/prices + snapshot-on-sync
│   │   ├── order.go            — PlaceOrder/Cancel/Get/List (credential_id, amount_pct)
│   │   ├── smart_order.go      — Create/List/Get/Cancel (credential_id)
│   │   ├── credential_group.go — CRUD credential groups + members
│   │   ├── bot.go              — Create/List/Get/Start/Stop/Delete
│   │   ├── analytics.go        — Summary/Coins/Snapshots/TakeSnapshot/Arbitrage
│   │   ├── dca.go              — Create/List/Get/Start/Stop/Delete
│   │   ├── futures.go          — GET /api/futures/positions
│   │   ├── spot_trades.go      — GET /api/spot-trades
│   │   └── ai.go               — POST /api/ai/ask (Claude claude-haiku-4-5)
│   ├── services/
│   │   ├── creds.go            — GetUserCreds() shared helper
│   │   ├── encryption.go       — AES-256-GCM encrypt/decrypt
│   │   ├── sync_service.go     — SyncUser/SyncAllUsers + SyncSpotTrades + SyncFutures
│   │   ├── sync_repository.go  — SQL для синку (upsert, cleanup, spot_trades, futures)
│   │   ├── price_service.go    — UpdateAllAssets() + GetLivePrices() + GetLivePricesByExchange()
│   │   ├── smart_order_service.go — CheckAndTrigger: SL/TP/Trailing
│   │   ├── bot_service.go      — Start/Stop/CheckBots (10 s)
│   │   ├── analytics_service.go — TakeAllSnapshots/TakeSnapshotForUser + TradeSummary(+from/to+total_fees) + CoinPerf + Arbitrage
│   │   ├── dca_service.go      — Start/Stop/CheckAndBuy (5 min)
│   │   ├── top_symbols_service.go — TopSymbolsService (FetchTopSymbols via Binance 24h ticker, Redis TTL 1h)
│   │   └── exchange/           — 6 бірж: Binance/OKX/Bybit/Gate/Kraken/KuCoin
│   │       ├── interface.go    — Exchange interface + Balance/Position(+MarginType)/ClosedTrade(+Fee/NotionalUsd/OpenedAt)/SpotTrade + SpotTrader
│   │       ├── registry.go + client.go + helpers.go + parse.go + trader.go
│   │       ├── binance.go      — GetBalances/GetOpenPositions/GetClosedTrades/GetPrices/GetRecentTrades
│   │       ├── okx.go          — GetBalances/GetOpenPositions/GetClosedTrades(+lever-info fallback+ctVal lookup)/GetPrices/GetRecentTrades
│   │       ├── bybit.go        — GetBalances/GetOpenPositions/GetClosedTrades/GetPrices/GetRecentTrades
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
│       ├── server.go           — broadcast loop (2 s): positions + spotPrices + spotPricesByExchange + topSymbols; price-based PnL%
│       ├── handler.go          — Fiber WS upgrade
│       └── server_test.go      — unit тести roundFloat/formatLeverage
│
├── migrations/                 — SQL файли 000001–000014
│   ├── 000001_initial_schema   — users, assets, portfolios, positions, history, credentials
│   ├── 000002_orders           — orders table
│   ├── 000003_smart_orders     — smart_orders table
│   ├── 000004_bots             — bots + bot_grids tables
│   ├── 000005_analytics        — portfolio_snapshots table
│   ├── 000006_dca_bots         — dca_bots table
│   ├── 000007_credentials_label — label + api_key_hint колонки в external_api_credentials
│   ├── 000008_futures_positions — futures_positions table (DECIMAL(20,8), leverage INT, margin_type)
│   ├── 000009_spot_trades      — spot_trades table (buy/sell, fee, fee_asset, traded_at)
│   ├── 000010_snapshot_datetime    — snapshot_date DATE → snapshot_at DATETIME + unique key
│   ├── 000011_snapshot_no_unique   — drop unique constraint on portfolio_snapshots (INSERT замість UPSERT)
│   ├── 000012_snapshot_exchange    — exchange column в portfolio_snapshots + index
│   ├── 000013_history_leverage_fix — UPDATE SET leverage='0x' WHERE leverage='1x' AND exchange='okx' 
│   └── 000014_unified_orders       — credential_groups + credential_group_members tables; credential_id + leverage on orders/smart_orders
│
├── frontend/
│   ├── Dockerfile              — node:20-alpine → nginx:1.27-alpine
│   ├── nginx.conf              — SPA routing, /api+/ws+/health proxy → api-gateway:8080, resolver directive
│   ├── vite.config.ts          — manualChunks: vendor-react/query/charts/icons/axios
│   ├── vitest.config.ts        — vitest configuration for unit tests
│   └── src/
│       ├── App.tsx             — React.lazy (11 сторінок) + Suspense + route guards
│       ├── api.ts              — 45+ типізованих API функцій; SpotTrade, FuturesPosition, Snapshot
│       ├── ws.ts               — useWebSocket hook (auto-reconnect 3 s); spotPricesByExchange + topSymbols
│       ├── context/AuthContext.tsx — user/token/login/logout/updateUser (cancelled-flag cleanup)
│       ├── components/
│       │   ├── Layout.tsx      — sidebar (9 nav items, Smart Orders removed), user footer
│       │   ├── SymbolPanel.tsx — candlestick chart (lightweight-charts) + Binance klines + symbol search + favorites
│       │   ├── PriceChart.tsx  — recharts AreaChart + Binance klines API (15m/1h/4h/1d intervals)
│       │   ├── CoinModal.tsx   — live price modal з PriceChart (uses credential_id)
│       │   ├── AiAdvisor.tsx   — AI chat widget (POST /api/ai/ask, Claude Haiku)
│       │   └── ExchangeSelector.tsx — dropdown з активних credentials
│       ├── orders/             — unified order system
│       │   ├── types.ts        — discriminated union types for 8 order types
│       │   ├── schemas/index.ts — zod validation schemas (discriminated union)
│       │   ├── adapters/okx.ts — OKX API v5 adapter (maps to /trade/order + /trade/order-algo)
│       │   ├── components/
│       │   │   ├── OrderTypeSelector.tsx — 8-type selector UI
│       │   │   ├── OrderForm.tsx         — main form (react-hook-form + zod)
│       │   │   └── fields/index.tsx      — shared form fields
│       │   ├── components/subforms/      — 8 subform files (one per order type)
│       │   └── __tests__/
│       │       ├── schemas.test.ts       — 15 schema validation tests
│       │       └── okx.adapter.test.ts   — 8 OKX adapter tests
│       └── pages/              — Login, Register, Dashboard, Portfolio, Orders (unified),
│                                  GridBots, DCABots, Analytics, Settings,
│                                  FuturesPositions, SpotTrades
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
GET              /api/orders?credential_ids=1,2,3

POST/GET/DELETE  /api/smart-orders | /api/smart-orders/:id

POST   /api/credential-groups
GET    /api/credential-groups
DELETE /api/credential-groups/:id
PUT    /api/credential-groups/:id/members

POST/GET/DELETE  /api/bots | /api/bots/:id
POST             /api/bots/:id/start | /stop

GET    /api/analytics/summary | /coins | /snapshots?days=30 | /arbitrage?min_spread=0.5
GET    /api/analytics/chart?range=7d&exchange=all   — 15-хв бакети для графіку портфеля (range: 1d/7d/30d/90d/custom + from/to)
POST   /api/analytics/snapshot

POST/GET/DELETE  /api/dca | /api/dca/:id
POST             /api/dca/:id/start?buy_now=true | /stop

GET    /api/futures/positions?exchange=binance
GET    /api/spot-trades?exchange=binance&days=30

POST   /api/ai/ask   { "question": "..." } → { "answer": "..." }

GET    /ws   (WebSocket)
```

### WebSocket протокол

```json
// Клієнт → { "type": "auth", "token": "eyJ..." }
// Сервер → { "type": "auth_success", "user_id": 7 }
// Сервер кожні 2 с →
{
  "type": "update",
  "positions": [{ "symbol": "BTC", "side": "LONG", "exchange": "binance", "pnl": 241.5, "pnl_pct": 1.85, ... }],
  "spot_prices": { "BTC": 67420, "ETH": 3210 },
  "spot_prices_by_exchange": { "binance": { "BTC": 67420 }, "okx": { "BTC": 67415 } },
  "top_symbols": ["BTC", "ETH", "SOL", "BNB", "XRP", ...]
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

**Фази 0–17 завершені.** Проєкт повністю функціональний.

Найближчі пріоритети:
- **Portfolio Value chart** — потребує 2+ snapshot-рядків у `portfolio_snapshots`; scheduler пише 1 раз/день, тому графік з'явиться природно через кілька днів
- **Spot Trades сторінка** — `SpotTrades.tsx` реалізована, але `SyncSpotTrades` можна додатково протестувати з реальними ключами

Можливі напрямки:
- **E2E тести** — handlers через `net/http/httptest` або `testcontainers`
- **Окрема БД на сервіс** — повна ізоляція (окремі MySQL схеми/інстанси)
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

### Фаза 16.1 — Futures Positions + Auto-Discovery ✅
- [x] Міграція 000008 — таблиця `futures_positions` (DECIMAL(20,8), leverage INT, margin_type)
- [x] `exchange.Position` — додано поле `MarginType`; Binance/OKX/Bybit заповнюють його
- [x] `internal/models/futures_position.go` — `FuturesPosition` + `FuturesPositionRepository` (Upsert/GetByUser/DeleteStale)
- [x] `SyncService.SyncFuturesForUser/SyncFuturesAllUsers` — паралельний синк в `futures_positions`
- [x] `DeleteStale` — видаляє закриті позиції (не повернуті Exchange)
- [x] `GET /api/futures/positions?exchange=binance` — handler + route в trading
- [x] `api.All("/futures*", trProxy)` — проксі в api-gateway
- [x] Auto-discovery: `PortfolioHandler.SetCredentialHook` → async goroutine після `AddCredential`
- [x] Scheduler `futures-sync` (30 s) у trading service
- [x] Frontend: `FuturesPositions.tsx` — таблиця + total PnL + рефреш
- [x] `ExchangeSelector.tsx` — dropdown з активних credentials
- [x] `api.ts` — `FuturesPosition`, `FuturesResponse`, `getFuturesPositions`
- [x] `Layout.tsx` — nav item "Futures" з іконкою Layers
- [x] `App.tsx` — lazy route `/futures`

### Фаза 16.2 — Spot Trades History ✅
- [x] Міграція 000009 — таблиця `spot_trades` (buy/sell, fee, fee_asset, traded_at ms, UNIQUE KEY)
- [x] `exchange.SpotTrader` interface — `GetRecentTrades(creds, startMs, endMs) ([]SpotTrade, error)`
- [x] `exchange.SpotTrade` type — Symbol/Side/Quantity/Price/Fee/FeeAsset/TradedAt
- [x] Binance/OKX/Bybit реалізують `SpotTrader` (`var _ SpotTrader = (*Binance)(nil)`)
- [x] `internal/models/spot_trade.go` — `SpotTrade` + `SpotTradeRepository` (Upsert/GetByUser)
- [x] `SyncService.SyncSpotTrades` — type assertion `SpotTrader`, синк за N днів
- [x] `GET /api/spot-trades?exchange=binance&days=30` — handler + route в trading
- [x] `api.All("/spot-trades*", trProxy)` — проксі в api-gateway
- [x] Frontend: `SpotTrades.tsx` — таблиця угод + exchange/days фільтри
- [x] `api.ts` — `SpotTrade`, `getSpotTrades`
- [x] `Layout.tsx` — nav item "Spot Trades"
- [x] `App.tsx` — lazy route `/spot-trades`

### Фаза 16.3 — AI Advisor ✅
- [x] `internal/handlers/ai.go` — `POST /api/ai/ask` → Anthropic Claude claude-haiku-4-5 API
- [x] System prompt з контекстом трейдинг-платформи; `question` → `answer`
- [x] `ANTHROPIC_KEY` в config + docker-compose
- [x] Frontend: `AiAdvisor.tsx` — collapsible chat widget (правий нижній кут)
- [x] `api.ts` — `askAi(question)` → `{ answer: string }`
- [x] `Layout.tsx` — AiAdvisor монтується глобально для всіх сторінок

### Фаза 16.4 — Portfolio Overhaul ✅
- [x] `Portfolio.tsx` — Balances + Positions + History tabs з exchange filter
- [x] `DetailModal` — відкривається по кліку на актив: stats + PriceChart
- [x] `PriceChart.tsx` — recharts AreaChart + Binance klines API (15m/1h/4h/1d) замість TradingView
- [x] `CoinModal.tsx` — live price popup з PriceChart (замість TradingView embed)
- [x] `PATCH /api/portfolio/assets/:id/price` — ручне оновлення avg_buy_price
- [x] `api.ts` — `updateAssetPrice`, `Snapshot` type

### Фаза 16.5 — Dashboard Redesign ✅
- [x] Exchange filter (All / Binance / OKX / Bybit) — фільтрує assets, positions, prices
- [x] Live Spot Prices grid — top 20 символів по 24h об'єму (TopSymbolsService + Redis)
- [x] `TopSymbolsService` — Binance `/api/v3/ticker/24hr`, сортування по quoteVolume, Redis TTL 1h
- [x] Coin search + `CoinModal` при кліку
- [x] `ws.ts` — отримує `spot_prices_by_exchange` + `top_symbols` з WS
- [x] WS `UpdateMessage` — додано `SpotPricesByExchange`, `TopSymbols`
- [x] `market-data` scheduler: `top-symbols-refresh` кожну годину
- [x] `GET /api/market/top-symbols` — HTTP fallback для TopSymbols

### Фаза 16.6 — Analytics Scheduler + PnL% Fix ✅
- [x] `analytics/main.go` — snapshot scheduler (1 h) + startup goroutine `TakeAllSnapshots`
- [x] `SyncHandler.FullSync` — тригерить `TakeSnapshotForUser` після синку (async)
- [x] `market-data/main.go` — `analyticsService` + snapshot-on-sync в `SyncHandler`
- [x] WS `server.go` — PnL% формула виправлена: `(markPx−entryPx)/entryPx × lev × 100`
  - Стара (невірна): `pnl / (entryPrice × qty / lev)` — ламається на inverse контрактах
  - Нова (вірна): price-based, не залежить від розміру контракту
- [x] `PriceService.GetLivePricesByExchange()` — ціни по біржах для WS broadcast

### Фаза 16.7 — History Data Quality Fix ✅
- [x] `ClosedTrade` struct — додано `Fee float64`, `NotionalUsd float64`, `OpenedAt int64`
- [x] `okx.GetClosedTrades` — парсинг `fee`, `cTime` (opened_at), `notionalUsd`
- [x] `okx.GetClosedTrades` — leverage fallback: `/api/v5/account/leverage-info` для cross-margin (lever=0)
- [x] `okx.GetClosedTrades` — ctVal lookup: `/api/v5/public/instruments` для коректного notional (qty в контрактах ≠ qty в монетах)
- [x] `sync_service.processHistory` — збереження `Fee`, `OpenedAt`, `MaxSize = NotionalUsd || qty×price×ctVal`
- [x] `sync_repository.insertHistory` — ON DUPLICATE KEY UPDATE: `leverage` (sentinel "0x"), `fee`, `opened_at`, `max_size`
- [x] Міграція `000013` — виправлення невірного "1x" → "0x" для OKX cross-margin записів
- [x] `analytics_service.GetTradeSummary` — підтримка `from`/`to` date params + `total_fees`
- [x] `handlers/analytics.go` — читання `from`/`to` query params
- [x] `api.ts` — `HistoryEntry.max_size`, `TradeSummary.total_fees`, `getSummary(from, to)`
- [x] `Portfolio.tsx` HistoryTab:
  - [x] R:R видалено з таблиці (залишено тільки в sidebar)
  - [x] Leverage sub-label завжди показується: "Cross" для невідомого cross-margin
  - [x] Margin = `e.max_size` (notionalUsd з БД; ctVal вже врахований бекендом)
  - [x] Sidebar: дата-фільтри (1d/7d/1m/3m preset + from/to date picker)
  - [x] Sidebar: R:R block (avgWin / |avgLoss|) з якісними мітками, Комісії, Profit Factor

### Фаза 17 — Unified Orders + Advanced Order System ✅
- [x] Міграція 000014 — `credential_groups`, `credential_group_members` tables; dropped unique constraint on `external_api_credentials`; added `credential_id` + `leverage` to `orders` and `smart_orders`
- [x] Credential Groups — API key grouping for multi-account order execution
- [x] Unified Orders page — merged Orders + Smart Orders into one page; replaced exchange selector with credential selector
- [x] 8-type Order Form — limit, market, TP/SL (OCO), chase, advanced limit (post_only/FOK/IOC), trailing stop, trigger, scaled (TWAP)
- [x] SymbolPanel — candlestick chart (lightweight-charts) with Binance klines, symbol search, favorites (localStorage)
- [x] Zod validation schemas — discriminated union schemas for all 8 order types
- [x] OKX API v5 adapter — maps internal order model to `/trade/order` and `/trade/order-algo` endpoints
- [x] Frontend packages — zod, react-hook-form, @hookform/resolvers, lightweight-charts, vitest
- [x] Unit tests — 23 tests (15 schema + 8 adapter tests)
- [x] Size % mode — orders can specify amount as % of deposit (`amount_pct`)
- [x] Nginx DNS fix — resolver directive to prevent stale IP caching
- [x] Settings.tsx — credential groups management section

### Фаза 17.1 — Auto-Sync (Live + Deep) ✅
- [x] Config: `SyncLiveInterval` (default 45s), `SyncDeepInterval` (default 15m) via env vars
- [x] NATS topics: `gateway.active-users` (heartbeat every 10s from api-gateway), `portfolio.synced` (per-user notification after sync)
- [x] Hub.ActiveUserIDs() + Hub.SendToUser() — exported methods for NATS integration
- [x] api-gateway: publishes active user IDs via NATS; subscribes to `portfolio.synced` → pushes `{type:"synced"}` to user WS
- [x] market-data scheduler: `sync-live` (positions+balances for active WS users), `sync-deep` (full SyncAllUsers for all)
- [x] Frontend ws.ts: handles `synced` message → invalidates portfolio/futures/history/summary/chart queries
- [x] Dashboard: "Sync All" → "Refresh now"; removed 15-minute references from chart hints

### Фаза 17.2 — History Margin Fix ✅
- [x] Migration 000015: `margin DECIMAL(20,8)` column in `position_history`
- [x] Added `MarginMode` field to `exchange.ClosedTrade`; filled in OKX/Binance/Bybit adapters
- [x] Removed hardcoded `"cross"` in `processPositions` (now uses `p.MarginType`) and `processHistory` (now uses `t.MarginMode`)
- [x] Compute real margin = `maxSize / leverage` during sync, stored in DB
- [x] Frontend: `HistoryEntry.margin` field; Portfolio History table shows "Size" (notional) + "Маржа" (allocated margin + margin mode label)
- [x] Bybit adapter: now parses leverage from closed-pnl endpoint

### Фаза 18 — Майбутнє
- [ ] **Forgot Password / Reset Password** — endpoint `POST /api/auth/forgot-password` (скидання пароля за email), сторінка на фронтенді; наразі відновлення можливе лише вручну через БД
- [ ] E2E тести (httptest / testcontainers)
- [ ] Окрема БД на сервіс (повна ізоляція)
- [ ] gRPC замість HTTP proxy для внутрішньої комунікації
- [ ] Service discovery (Consul або env-based)
- [ ] Stripe підписки (Free / Pro / Enterprise)
- [ ] `GetAvailableSymbols` + валідація ордерів по реальних парах
- [ ] Portfolio Value chart (потребує 2+ днів snapshot даних)

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

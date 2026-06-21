# TradeTracker Go — Архітектура системи

## Загальний огляд

TradeTracker Go — мультибіржовий портфельний трекер з торговими ботами, smart orders та аналітикою.
Backend: Go + Fiber v2 + MySQL. Frontend: React 18 + Vite + TypeScript + Tailwind.

```
┌────────────────────────────────────────────────────────────────┐
│                         Клієнт                                 │
│           React SPA (nginx :80)  /  curl / Postman             │
└────────────┬──────────────────────────────────┬────────────────┘
             │ HTTP/REST (:8080)                 │ WebSocket /ws
             ▼                                  ▼
┌────────────────────────────────────────────────────────────────┐
│                    Fiber v2 HTTP Server                        │
│  Auth │ Portfolio │ Orders │ SmartOrders │ Bots │ DCA │ Analytics │
│  ──────────────── JWT Middleware + Rate Limiter ────────────── │
└──────┬──────────────────────┬───────────────────┬─────────────┘
       │                      │                   │
       ▼                      ▼                   ▼
┌─────────────┐  ┌─────────────────────┐  ┌────────────────────┐
│  Repository │  │    Service Layer    │  │  Exchange Adapters │
│  Layer      │  │ Sync │ Price │ Enc  │  │  Binance/OKX/Bybit │
│  (sqlx/SQL) │  │ SmartOrder │ Bot   │  │  Gate/Kraken/KuCoin│
│             │  │ DCA │ Analytics     │  └────────────────────┘
└──────┬──────┘  └──────────┬──────────┘
       │                    │
       ▼                    ▼
┌────────────┐   ┌──────────────────────────────────────────────┐
│   MySQL    │   │            Background Scheduler              │
│  (pool 25) │   │  prices(30s) │ smart-orders(5s) │ bots(10s) │
└────────────┘   │  dca(5min)  │ sync-all-users(15min)         │
                 └──────────────────────────────────────────────┘
┌────────────┐   ┌───────────────┐   ┌─────────────────────────┐
│   Redis    │   │  WS Hub/Server│   │  Prometheus /metrics    │
│  (optional)│   │  broadcast(2s)│   │  Telegram notify        │
└────────────┘   └───────────────┘   └─────────────────────────┘
```

---

## Структура директорій

```
tradetracker-go/
├── cmd/
│   ├── server/main.go          — точка входу: DI, роути, scheduler, graceful shutdown
│   └── migrate/main.go         — CLI: up/down/version/force
│
├── internal/
│   ├── config/config.go        — .env → Config struct
│   ├── database/db.go          — sqlx connect, pool max 25
│   ├── middleware/auth.go      — JWT: Bearer + cookie, GetUserID()
│   │
│   ├── models/                 — структури + репозиторії
│   │   ├── user.go             — User + UserRepository
│   │   ├── portfolio.go        — Asset, UserPortfolio, Position, History, Credentials
│   │   ├── order.go            — Order + OrderRepository
│   │   ├── smart_order.go      — SmartOrder + SmartOrderRepository
│   │   ├── bot.go              — Bot + BotGrid + BotRepository
│   │   ├── snapshot.go         — PortfolioSnapshot + SnapshotRepository
│   │   ├── dca_bot.go          — DCABot + DCABotRepository
│   │   ├── testmain_test.go    — TestMain: авто-DB + migrate UP + truncateAll
│   │   ├── user_repo_test.go   — 4 інтеграційні тести
│   │   ├── order_repo_test.go  — 5 інтеграційні тести
│   │   ├── smart_order_repo_test.go — 7 інтеграційні тести
│   │   ├── bot_repo_test.go    — 6 інтеграційні тести (grid lifecycle)
│   │   ├── dca_bot_repo_test.go — 6 інтеграційні тести
│   │   └── snapshot_repo_test.go — 4 інтеграційні тести
│   │
│   ├── handlers/               — HTTP handlers (Transport Layer)
│   │   ├── auth.go             — register/login/logout/me
│   │   ├── portfolio.go        — CRUD credentials + comments
│   │   ├── sync.go             — full/positions/history/prices sync
│   │   ├── order.go            — PlaceOrder/CancelOrder/GetOrder/List
│   │   ├── smart_order.go      — Create/List/Get/Cancel
│   │   ├── bot.go              — Create/List/Get/Start/Stop/Delete
│   │   ├── analytics.go        — Summary/Coins/Snapshots/Arbitrage
│   │   └── dca.go              — Create/List/Get/Start/Stop/Delete
│   │
│   ├── services/
│   │   ├── creds.go            — GetUserCreds() — shared helper (decrypt API keys)
│   │   ├── encryption.go       — AES-256-GCM encrypt/decrypt
│   │   ├── sync_service.go     — паралельний синк + SyncAllUsers()
│   │   ├── sync_repository.go  — SQL для синку (upsert, cleanup, transfer)
│   │   ├── price_service.go    — UpdateAllAssets() через PriceStorer
│   │   ├── smart_order_service.go — CheckAndTrigger кожні 5с (SL/TP/Trailing)
│   │   ├── bot_service.go      — Start/Stop/CheckBots кожні 10с
│   │   ├── analytics_service.go — TakeSnapshot/TradeSummary/CoinPerf/Arbitrage
│   │   ├── dca_service.go      — Start/Stop/CheckAndBuy кожні 5хв
│   │   └── exchange/
│   │       ├── interface.go + registry.go + client.go + helpers.go + parse.go
│   │       ├── trader.go       — Trader interface (PlaceOrder/Cancel/Status)
│   │       ├── binance.go + binance_trade.go
│   │       ├── okx.go + okx_trade.go
│   │       ├── bybit.go + bybit_trade.go
│   │       ├── gate.go / kraken.go / kucoin.go
│   │       ├── helpers_test.go — unit тести hmac/sha512/normalizeSymbol
│   │       └── parse_test.go   — unit тести parseFloat/parseInt64
│   │
│   ├── cache/
│   │   ├── cache.go            — PriceStorer interface
│   │   ├── memory.go           — MemoryPriceStore (TTL 30s, sync.RWMutex)
│   │   └── redis.go            — RedisPriceStore (авто через REDIS_URL)
│   │
│   ├── scheduler/
│   │   └── scheduler.go        — background jobs з context cancellation
│   │
│   ├── metrics/
│   │   └── metrics.go          — HTTP counter/histogram, WS gauge, scheduler histogram
│   │
│   ├── notify/
│   │   └── telegram.go         — Telegram Bot API (no-op якщо токен не задано)
│   │
│   ├── validator/
│   │   └── validator.go        — обгортка go-playground/validator
│   │
│   └── ws/
│       ├── hub.go              — реєстр WS-з'єднань (sync.RWMutex)
│       ├── client.go           — ReadPump + WritePump goroutines
│       ├── server.go           — broadcast loop (кожні 2с)
│       ├── handler.go          — Fiber WS upgrade
│       └── server_test.go      — unit тести roundFloat/formatLeverage
│
├── migrations/                 — SQL файли (000001–000006)
│   ├── 000001_initial_schema   — users, assets, portfolios, positions, history, credentials
│   ├── 000002_orders           — orders table
│   ├── 000003_smart_orders     — smart_orders table
│   ├── 000004_bots             — bots + bot_grids tables
│   ├── 000005_snapshots        — portfolio_snapshots table
│   └── 000006_dca_bots         — dca_bots table
│
├── frontend/
│   ├── Dockerfile              — node:20-alpine → nginx:1.27-alpine
│   ├── nginx.conf              — SPA routing, /api proxy, /ws WS upgrade, asset caching
│   ├── .dockerignore
│   ├── vite.config.ts          — manualChunks: vendor-react/query/charts/icons/axios
│   └── src/
│       ├── App.tsx             — React.lazy (9 сторінок) + Suspense spinner
│       ├── api.ts              — 30+ типізованих API функцій
│       ├── ws.ts               — useWebSocket hook (auto-reconnect 3s)
│       ├── context/AuthContext.tsx
│       ├── components/Layout.tsx
│       └── pages/              — Login, Register, Dashboard, Portfolio, Orders,
│                                  SmartOrders, GridBots, DCABots, Analytics
│
├── Dockerfile                  — golang:1.26-alpine → alpine:3.20
├── docker-compose.yml          — db, redis, migrate, api, frontend
├── .dockerignore
└── docs/                       — ця документація
```

---

## Шари архітектури

### 1. Transport Layer (handlers/)
- Валідує вхід через `validator.Validate()`
- Витягує `userID` з JWT через `middleware.GetUserID(c)`
- Викликає сервіс або репозиторій
- Повертає JSON відповідь

### 2. Service Layer (services/)
- **GetUserCreds** — shared helper для розшифрування API credentials (4 callers)
- **SyncService** — паралельний синк: goroutine на кожну біржу + WaitGroup
- **PriceService** — ціни через PriceStorer (memory або Redis)
- **SmartOrderService** — CheckAndTrigger: SL/TP/TrailingStop кожні 5с
- **BotService** — Grid bot: Start/Stop/CheckBots кожні 10с, counter orders
- **DCAService** — CheckAndBuy кожні 5хв, qty = amount_usd / live_price
- **AnalyticsService** — snapshots, winrate, profit_factor, arbitrage scanner

### 3. Exchange Adapters (services/exchange/)
Два інтерфейси:
- **Exchange** — читання: GetBalances, GetPositions, GetClosedTrades, GetLivePrices
- **Trader** (опціональний) — торгівля: PlaceOrder, CancelOrder, GetOrderStatus

Trader реалізують лише Binance, OKX, Bybit. Gate.io, Kraken, KuCoin — тільки Exchange.

```go
ex, _ := registry.Get(req.Exchange)
trader, ok := ex.(exchange.Trader)   // type assertion — безпечно, не panic
if !ok {
    return c.Status(400).JSON(fiber.Map{"error": "trading not supported for this exchange"})
}
```

### 4. Repository Layer (models/)
Прямий SQL через `sqlx`. Без ORM.
- `INSERT IGNORE` / `ON DUPLICATE KEY UPDATE` для idempotency
- Нові таблиці — тільки через нову міграцію

### 5. Cache Layer (cache/)
`PriceStorer` interface → swap memory↔Redis без змін у PriceService.
Redis авто-активується через `REDIS_URL` в `.env`.

### 6. Notifications (notify/)
`Notifier` interface → no-op реалізація якщо `TELEGRAM_BOT_TOKEN` не задано.
Async goroutine в scheduler — не блокує основний потік.

### 7. Metrics (metrics/)
Prometheus counters/histograms — `/metrics` ендпоінт через `gofiber/adaptor`.

---

## База даних

**Engine:** MySQL 8.0
**БД:** `tradetracker` (локально) / `tradetracker_test` (інтеграційні тести)
**Pool:** max 25 з'єднань

| Таблиця | Призначення | Міграція |
|---|---|---|
| `users` | Акаунти | 000001 |
| `assets` | Відомі активи (34 seed) | 000001 |
| `user_portfolios` | Спотові баланси | 000001 |
| `open_positions` | Відкриті ф'ючерсні позиції | 000001 |
| `position_history` | Закриті угоди | 000001 |
| `external_api_credentials` | Зашифровані API ключі | 000001 |
| `orders` | Торгові ордери | 000002 |
| `smart_orders` | SL/TP/TrailingStop | 000003 |
| `bots` + `bot_grids` | Grid боти + рівні | 000004 |
| `portfolio_snapshots` | Щогодинні знімки | 000005 |
| `dca_bots` | DCA боти | 000006 |

---

## Конкурентність

- Синк: кожна біржа — окрема goroutine (`sync.WaitGroup`)
- WS клієнт: 2 goroutines (ReadPump + WritePump)
- Scheduler: goroutine per job з `context.Context` cancellation
- Hub: `sync.RWMutex` для реєстру клієнтів
- Memory cache: `sync.RWMutex`
- Redis: ізольований 200ms таймаутом

---

## Паттерни

### Dependency Injection
Всі залежності через конструктор. Глобальних змінних немає.

```go
enc         := services.NewEncryptionService(cfg.EncryptionKey)
priceStore  := cache.New(cfg.RedisURL, logger)
priceService := services.NewPriceService(db, priceStore, logger)
orderHandler := handlers.NewOrderHandler(orderRepo, portfolioRepo, enc, logger)
```

### Shared Credentials Helper
```go
// internal/services/creds.go
func GetUserCreds(portfolio *models.PortfolioRepository, enc *EncryptionService,
    userID int64, exchangeName string) (exchange.Credentials, error)
```
Один хелпер замість 4 дубльованих методів у handlers та services.

### Graceful Shutdown
```
SIGINT/SIGTERM → cancel(ctx) → scheduler.Stop() → WS hub.Close() → app.ShutdownWithTimeout(5s) → db.Close()
```

### Code Splitting (Frontend)
```ts
// Кожна сторінка — окремий chunk (3–11 kB)
const Dashboard = lazy(() => import('./pages/Dashboard'))

// Vendor chunks (manualChunks у vite.config.ts)
// vendor-react 158kB, vendor-charts 393kB, ...
```

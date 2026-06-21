# TradeTracker Go — Архітектура системи

## Загальний огляд

TradeTracker Go — мультибіржовий портфельний трекер з торговими ботами, smart orders та аналітикою.
Архітектура: **4 мікросервіси** на Go + Fiber v2, з'єднані через HTTP reverse-proxy (api-gateway)
і NATS message bus для асинхронних подій. Frontend: React 18 + Vite + TypeScript + Tailwind.

---

## Топологія мікросервісів

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                            Клієнт / Браузер                                  │
│                    React SPA (nginx :3000)  /  curl / Postman                │
└────────────────────────────────────┬─────────────────────────────────────────┘
                                     │ HTTP :3000
                                     ▼
                          ┌────────────────────┐
                          │   nginx (frontend) │  статичні файли + proxy
                          │   /api → :8080     │
                          │   /ws  → :8080 WS  │
                          └─────────┬──────────┘
                                    │
                                    ▼
            ┌───────────────────────────────────────────────┐
            │              api-gateway  :8080               │
            │                                               │
            │  • JWT validation + rate limiting             │
            │  • Auth handlers (register/login/me/profile)  │
            │  • WebSocket hub (broadcast loop 2 s)         │
            │  • HTTP reverse-proxy → downstream services   │
            │  • Prometheus /metrics                        │
            └────────┬──────────────┬───────────────┬───────┘
                     │              │               │
           /api/sync/*    /api/portfolio|orders    /api/analytics/*
           /api/*sync     |bots|dca|smart-orders/*
                     │              │               │
                     ▼              ▼               ▼
          ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐
          │ market-data │  │   trading    │  │   analytics     │
          │   :8081     │  │    :8082     │  │     :8083       │
          │             │  │              │  │                 │
          │ • Sync APIs │  │ • Portfolio  │  │ • Summary       │
          │ • Price sched│  │ • Orders     │  │ • Coins perf.  │
          │   (30 s)    │  │ • Smart Ord. │  │ • Snapshots     │
          │ • Redis write│  │ • Grid Bots  │  │ • Arbitrage     │
          └──────┬──────┘  │ • DCA Bots   │  └─────────────────┘
                 │          │ • Schedulers │
                 │ NATS     └──────────────┘
                 │ market.prices.updated
                 └──────────────────────────────► trading (smart-order trigger)

┌──────────────┐  ┌──────────────┐  ┌─────────────────────────────────────────┐
│  MySQL :3306 │  │  Redis :6379 │  │  NATS :4222                             │
│  (shared DB) │  │ (price cache)│  │  subject: market.prices.updated         │
└──────────────┘  └──────────────┘  └─────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────────┐
│  Monitoring                                                                  │
│  Prometheus :9090  (4 scrape targets)  →  Grafana :3001  (TradeTracker dash) │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## Сервіси

### api-gateway (:8080)

Єдина точка входу ззовні. Нічого не знає про бізнес-логіку — лише маршрутизує.

| Відповідальність | Деталі |
|---|---|
| JWT validation | Bearer token + cookie fallback; після валідації інжектить `X-Internal-User-ID` |
| Rate limiting | 10 req/min per IP для /auth/register та /auth/login |
| Auth handlers | `POST /api/auth/{register,login,logout}`, `GET /api/auth/me` |
| Profile handlers | `PATCH /api/user/profile`, `PATCH /api/user/password` |
| WebSocket | Hub + Server broadcast loop 2 s; читає ціни з Redis (записані market-data) |
| HTTP proxy | Reverse-proxy до market-data / trading / analytics з форвардингом user ID |
| Prometheus | `/metrics` endpoint |

**Proxy mapping:**

| Prefix | Upstream |
|---|---|
| `/api/sync/*` | `market-data:8081` |
| `/api/portfolio*`, `/api/positions/*`, `/api/history/*` | `trading:8082` |
| `/api/orders*`, `/api/smart-orders*`, `/api/bots*`, `/api/dca*` | `trading:8082` |
| `/api/analytics*` | `analytics:8083` |

---

### market-data (:8081)

Власник всього, що стосується ринкових даних і синхронізації з біржами.

| Відповідальність | Деталі |
|---|---|
| Sync endpoints | `POST /api/sync/{full,positions,history}`, `GET /api/sync/prices` |
| Price scheduler | Кожні 30 с: `UpdateAllAssets()` → MySQL + Redis, потім publish NATS `market.prices.updated` |
| User sync scheduler | Кожні 15 хв: `SyncAllUsers()` — паралельний синк усіх зареєстрованих юзерів |
| Auth | `InternalAuth` middleware — читає `X-Internal-User-ID` (не валідує JWT) |

---

### trading (:8082)

Власник всієї торгової логіки та автоматизації.

| Відповідальність | Деталі |
|---|---|
| Portfolio | `GET /api/portfolio/{/,history,credentials}`, `POST/DELETE credentials` |
| Orders | `POST/GET/DELETE /api/orders` |
| Smart Orders | `POST/GET/DELETE /api/smart-orders` |
| Grid Bots | `POST/GET/Start/Stop/Delete /api/bots` |
| DCA Bots | `POST/GET/Start/Stop/Delete /api/dca` |
| Smart-order scheduler | Кожні 30 с (fallback); миттєво при NATS `market.prices.updated` |
| Grid-bot scheduler | Кожні 10 с |
| DCA scheduler | Кожні 5 хв |
| Snapshot scheduler | Щогодини |
| NATS subscribe | `market.prices.updated` → `smartOrderService.CheckAndTrigger()` |

---

### analytics (:8083)

Тільки читання — аналітика по торговій історії.

| Відповідальність | Деталі |
|---|---|
| Trade summary | `GET /api/analytics/summary` — winrate, profit_factor, avg_pnl |
| Coin performance | `GET /api/analytics/coins` |
| Snapshots | `GET /api/analytics/snapshots`, `POST /api/analytics/snapshot` |
| Arbitrage | `GET /api/analytics/arbitrage?min_spread=0.5` |

---

## NATS Message Bus

**Брокер:** NATS 2 (`:4222`). Lightweight pub/sub без персистентності.

### Топік `market.prices.updated`

```go
type PricesMsg struct {
    Prices map[string]float64 `json:"prices"`
    At     time.Time          `json:"at"`
}
```

| Роль | Дія |
|---|---|
| **Publisher** | `market-data` — кожні 30 с після оновлення Redis |
| **Subscriber** | `trading` — викликає `CheckAndTrigger()` для smart orders |

**Моніторинг:** `http://localhost:8222` — NATS HTTP monitoring UI.

---

## Auth між сервісами

```
Клієнт → api-gateway:  Authorization: Bearer <JWT>
                              ↓ validate
api-gateway → trading:  X-Internal-User-ID: 42
trading:  middleware.InternalAuth() → c.Locals("user_id", 42)
```

Downstream сервіси (market-data, trading, analytics) не мають JWT secret.
Вони довіряють хедеру `X-Internal-User-ID`, який виставляє gateway.
Це безпечно, бо downstream сервіси не доступні ззовні.

---

## Структура директорій

```
tradetracker-go/
│
├── cmd/
│   ├── api-gateway/main.go     — шлюз: JWT, WS, proxy, auth handlers
│   ├── market-data/main.go     — ціни, синк, NATS publish
│   ├── trading/main.go         — портфель, ордери, боти, NATS subscribe
│   ├── analytics/main.go       — аналітика
│   ├── server/main.go          — монолітна точка входу (legacy, не використовується)
│   └── migrate/main.go         — CLI для міграцій (up/down/version/force)
│
├── internal/                   — всі пакети є shared між сервісами
│   │
│   ├── config/config.go        — .env → Config struct
│   │                             (додано: NatsURL, MarketDataURL, TradingURL, AnalyticsURL)
│   ├── database/db.go          — sqlx connect, pool max 25
│   │
│   ├── middleware/
│   │   ├── auth.go             — JWTAuth(): Bearer + cookie; GetUserID()
│   │   └── internal.go         — InternalAuth(): X-Internal-User-ID → Locals
│   │
│   ├── nats/
│   │   └── bus.go              — Bus wrapper (Connect, Publish, Subscribe, Close)
│   │                             + SubjPricesUpdated const + PricesMsg type
│   │
│   ├── models/                 — структури + репозиторії (shared між усіма сервісами)
│   │   ├── user.go             — User + UserRepository (Create, Find, UpdateProfile, UpdatePassword)
│   │   ├── portfolio.go        — Asset, Position, History, ExternalApiCredential (label, api_key_hint), PortfolioRepository
│   │   ├── order.go            — Order + OrderRepository
│   │   ├── smart_order.go      — SmartOrder + SmartOrderRepository
│   │   ├── bot.go              — Bot + BotGrid + BotRepository
│   │   ├── snapshot.go         — PortfolioSnapshot + SnapshotRepository
│   │   ├── dca_bot.go          — DCABot + DCABotRepository
│   │   └── *_test.go           — 33 інтеграційні тести
│   │
│   ├── handlers/               — HTTP handlers (Transport Layer, shared)
│   │   ├── auth.go             — Register/Login/Logout/Me/UpdateProfile/ChangePassword
│   │   ├── portfolio.go        — CRUD credentials (label, api_key_hint), comments
│   │   ├── sync.go             — full/positions/history/prices
│   │   ├── order.go            — PlaceOrder/CancelOrder/GetOrder/List
│   │   ├── smart_order.go      — Create/List/Get/Cancel
│   │   ├── bot.go              — Create/List/Get/Start/Stop/Delete
│   │   ├── analytics.go        — Summary/Coins/Snapshots/Arbitrage
│   │   └── dca.go              — Create/List/Get/Start/Stop/Delete
│   │
│   ├── services/
│   │   ├── creds.go            — GetUserCreds() shared helper
│   │   ├── encryption.go       — AES-256-GCM encrypt/decrypt
│   │   ├── sync_service.go     — паралельний синк + SyncAllUsers()
│   │   ├── sync_repository.go  — SQL для синку
│   │   ├── price_service.go    — UpdateAllAssets() + GetLivePrices() через PriceStorer
│   │   ├── smart_order_service.go — CheckAndTrigger: SL/TP/Trailing
│   │   ├── bot_service.go      — Start/Stop/CheckBots
│   │   ├── analytics_service.go — Snapshot/TradeSummary/CoinPerf/Arbitrage
│   │   ├── dca_service.go      — Start/Stop/CheckAndBuy
│   │   └── exchange/           — 6 бірж: Binance, OKX, Bybit, Gate, Kraken, KuCoin
│   │       ├── interface.go    — Exchange interface: GetBalances/Positions/Prices
│   │       ├── trader.go       — Trader interface: PlaceOrder/Cancel/Status
│   │       ├── registry.go     — map[name]Exchange
│   │       ├── binance.go + binance_trade.go
│   │       ├── okx.go + okx_trade.go
│   │       ├── bybit.go + bybit_trade.go
│   │       └── gate.go / kraken.go / kucoin.go
│   │
│   ├── cache/
│   │   ├── cache.go            — PriceStorer interface
│   │   ├── memory.go           — in-memory (TTL 30 s, sync.RWMutex)
│   │   └── redis.go            — Redis (shared між api-gateway і market-data)
│   │
│   ├── scheduler/
│   │   └── scheduler.go        — background jobs з context.Context cancellation
│   │
│   ├── metrics/
│   │   └── metrics.go          — HTTP counter/histogram, WS/bots/smart-orders gauges
│   │
│   ├── notify/
│   │   └── telegram.go         — Telegram Bot API (no-op без токена)
│   │
│   ├── validator/
│   │   └── validator.go        — Validate() helper
│   │
│   └── ws/
│       ├── hub.go              — реєстр WS-з'єднань (sync.RWMutex)
│       ├── client.go           — ReadPump + WritePump goroutines
│       ├── server.go           — broadcast loop (2 s): positions + prices
│       └── handler.go          — Fiber WS upgrade
│
├── migrations/                 — SQL файли 000001–000007
│
├── frontend/
│   ├── Dockerfile              — node:20-alpine → nginx:1.27-alpine
│   ├── nginx.conf              — SPA routing, /api+/ws+/health proxy → api-gateway:8080
│   └── src/
│       ├── App.tsx             — React.lazy (10 сторінок) + Suspense
│       ├── api.ts              — 35+ типізованих API функцій
│       ├── ws.ts               — useWebSocket hook (auto-reconnect 3 s)
│       ├── context/AuthContext.tsx — user, token, login/logout/updateUser
│       ├── components/Layout.tsx   — sidebar з 8 nav items
│       └── pages/              — Login, Register, Dashboard, Portfolio, Orders,
│                                  SmartOrders, GridBots, DCABots, Analytics, Settings
│
├── monitoring/
│   ├── prometheus.yml          — 4 scrape targets (api-gateway, market-data, trading, analytics)
│   └── grafana/
│       ├── provisioning/       — auto-provisioning datasource + dashboard
│       └── dashboards/tradetracker.json  — 10 panels: HTTP rate, latency, WS clients, bots
│
├── Dockerfile                  — один image, 5 бінарників (api-gateway, market-data, trading, analytics, migrate)
└── docker-compose.yml          — 10 сервісів: db, redis, nats, migrate, 4×Go, frontend, prometheus, grafana
```

---

## Шари архітектури

### 1. Transport Layer (handlers/)

Shared між усіма сервісами. Валідує вхід, витягує `userID`, викликає сервіс/репозиторій.

```go
func (h *OrderHandler) PlaceOrder(c *fiber.Ctx) error {
    userID := middleware.GetUserID(c)  // читає c.Locals("user_id") — встановлено
                                        // JWTAuth (gateway) або InternalAuth (trading)
    // ...
}
```

### 2. Service Layer (services/)

Бізнес-логіка. Незалежна від HTTP.

- **SyncService** — паралельний синк: goroutine на кожну біржу + WaitGroup
- **PriceService** — ціни через `PriceStorer` (memory або Redis)
- **SmartOrderService** — CheckAndTrigger: SL/TP/TrailingStop
- **BotService** — Grid bot: Start/Stop/CheckBots
- **DCAService** — CheckAndBuy: qty = amount_usd / live_price
- **AnalyticsService** — snapshots, winrate, profit_factor, arbitrage scanner

### 3. Exchange Adapters (services/exchange/)

Два інтерфейси:
- **Exchange** — читання: GetBalances, GetPositions, GetClosedTrades, GetPrices
- **Trader** (опціональний) — PlaceOrder, CancelOrder, GetOrderStatus

```go
trader, ok := ex.(exchange.Trader)  // type assertion
if !ok {
    return c.Status(400).JSON(fiber.Map{"error": "trading not supported"})
}
```

### 4. Repository Layer (models/)

Прямий SQL через `sqlx`. Без ORM. `INSERT IGNORE` / `ON DUPLICATE KEY UPDATE` для idempotency.

### 5. Cache Layer (cache/)

`PriceStorer` interface → swap memory↔Redis без змін у `PriceService`.

- **market-data** пише до Redis (через `priceService.UpdateAllAssets()`)
- **api-gateway WS** читає з Redis (через `priceService.GetLivePrices()`)
- Shared Redis = ціни доступні обом без NATS для WS

---

## База даних

**Engine:** MySQL 8.0 | **БД:** `tradetracker_go` | **Pool:** max 25 з'єднань на сервіс

| Таблиця | Власник (пише) | Міграція |
|---|---|---|
| `users` | api-gateway | 000001 |
| `assets` | market-data | 000001 |
| `user_portfolios` | trading, market-data | 000001 |
| `open_positions` | trading, market-data | 000001 |
| `position_history` | trading, market-data | 000001 |
| `external_api_credentials` | trading | 000001 |
| `orders` | trading | 000002 |
| `smart_orders` | trading | 000003 |
| `bots` + `bot_grids` | trading | 000004 |
| `portfolio_snapshots` | trading | 000005 |
| `dca_bots` | trading | 000006 |
| `external_api_credentials` (label, api_key_hint) | trading | 000007 |

> Shared database — прагматичний підхід для дипломного проєкту.
> Повна ізоляція БД (окрема схема/інстанс на сервіс) — природний наступний крок.

---

## Конкурентність

| Компонент | Паттерн |
|---|---|
| Exchange sync | goroutine per exchange + `sync.WaitGroup` |
| WS клієнт | 2 goroutines: ReadPump + WritePump |
| Scheduler jobs | goroutine per job, зупинка через `context.Done()` |
| WS Hub | `sync.RWMutex` для реєстру клієнтів |
| Memory price cache | `sync.RWMutex` |
| Redis | `200 ms` timeout |
| Smart-order trigger | NATS subscriber goroutine + fallback 30 s ticker |

---

## Паттерни

### Gateway Pattern
api-gateway — єдина публічна точка. Frontend знає тільки про `:8080`.
Downstream сервіси не мають публічних портів у production (можна прибрати `ports:` з compose).

### Internal Auth (Header Propagation)
```
JWT (external) → gateway validates → X-Internal-User-ID: 42 → downstream trusts header
```

### Pub/Sub (NATS)
Event-driven trigger: market-data → `market.prices.updated` → trading виконує smart-order check
без polling і без прямого HTTP між сервісами.

### Dependency Injection
Всі залежності через конструктор. Глобальних змінних немає.

### Graceful Shutdown
```
SIGTERM → cancel(ctx) → scheduler.Stop() → WS hub → app.ShutdownWithTimeout(5s) → db.Close()
```

### Code Splitting (Frontend)
```ts
const Dashboard = lazy(() => import('./pages/Dashboard'))  // окремий chunk 3–11 kB
```

---

## Docker Compose — порти

| Сервіс | Зовнішній порт | Призначення |
|---|---|---|
| frontend | 3000 | React SPA (nginx) |
| api-gateway | 8080 | Єдина публічна точка API + WS |
| market-data | 8081 | Внутрішній (sync endpoints) |
| trading | 8082 | Внутрішній (all trading) |
| analytics | 8083 | Внутрішній (stats) |
| db (MySQL) | 3306 | |
| redis | 6379 | |
| nats | 4222, 8222 | 4222 — клієнти; 8222 — HTTP monitoring |
| prometheus | 9090 | |
| grafana | 3001 | admin/admin |

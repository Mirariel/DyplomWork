# TradeTracker Go — Архітектура системи

## Загальний огляд

TradeTracker Go — мультибіржовий портфельний трекер з торговими можливостями.
Сервер написаний на Go, використовує Fiber v2 як HTTP-фреймворк і MySQL як сховище даних.

```
┌─────────────────────────────────────────────────────────┐
│                        Клієнт                           │
│          (браузер / мобільний додаток / CLI)            │
└───────────┬───────────────────────────────┬─────────────┘
            │ HTTP/REST                     │ WebSocket
            ▼                               ▼
┌─────────────────────────────────────────────────────────┐
│                   Fiber v2 HTTP Server                  │
│   ┌──────────┐  ┌──────────┐  ┌──────────────────────┐ │
│   │  Auth    │  │Portfolio │  │     Orders           │ │
│   │ Handler  │  │ Handler  │  │     Handler          │ │
│   └──────────┘  └──────────┘  └──────────────────────┘ │
│   ┌──────────┐  ┌──────────┐  ┌──────────────────────┐ │
│   │  Sync    │  │   WS     │  │  JWT Middleware       │ │
│   │ Handler  │  │ Handler  │  │  Rate Limiter        │ │
│   └──────────┘  └──────────┘  └──────────────────────┘ │
└──────┬──────────────┬─────────────────┬─────────────────┘
       │              │                 │
       ▼              ▼                 ▼
┌──────────────┐ ┌──────────┐  ┌──────────────────────┐
│  SyncService │ │  WS Hub  │  │   Exchange Adapters  │
│  PriceService│ │  Server  │  │  Binance/OKX/Bybit   │
│  EncService  │ └──────────┘  │  Gate/Kraken/KuCoin  │
└──────┬───────┘               └──────────────────────┘
       │
       ▼
┌──────────────┐  ┌──────────────────────────────────────┐
│    MySQL     │  │          Background Scheduler         │
│  (sqlx/raw)  │  │  prices(30s) | sync-all-users(15min) │
└──────────────┘  └──────────────────────────────────────┘
```

---

## Структура директорій

```
tradetracker-go/
├── cmd/
│   ├── server/main.go       — точка входу, DI, реєстрація роутів
│   └── migrate/main.go      — CLI для міграцій БД
│
├── internal/
│   ├── config/config.go     — зчитує .env у Config struct
│   ├── database/db.go       — sqlx підключення (pool max 25)
│   ├── middleware/auth.go   — JWT: Bearer + cookie, GetUserID()
│   │
│   ├── models/              — структури БД і репозиторії
│   │   ├── user.go          — User + UserRepository
│   │   ├── portfolio.go     — Asset, Position, History, Credentials
│   │   └── order.go         — Order + OrderRepository
│   │
│   ├── handlers/            — HTTP обробники
│   │   ├── auth.go          — register/login/logout/me
│   │   ├── portfolio.go     — CRUD credentials + comments
│   │   ├── sync.go          — тригери синхронізації
│   │   └── order.go         — розміщення/скасування/статус ордерів
│   │
│   ├── services/
│   │   ├── encryption.go    — AES-256-GCM encrypt/decrypt
│   │   ├── price_service.go — ціни активів з кешем
│   │   ├── sync_service.go  — паралельний синк по всіх біржах
│   │   ├── sync_repository.go — SQL для синку
│   │   └── exchange/        — адаптери бірж
│   │       ├── interface.go — Exchange interface
│   │       ├── trader.go    — Trader interface (торгівля)
│   │       ├── registry.go  — карта name→Exchange
│   │       ├── client.go    — HTTP client з retry
│   │       ├── helpers.go   — підпис HMAC, nowMs, normalizeSymbol
│   │       ├── parse.go     — parseFloat, parseInt64
│   │       ├── binance.go / binance_trade.go
│   │       ├── okx.go / okx_trade.go
│   │       ├── bybit.go / bybit_trade.go
│   │       ├── gate.go
│   │       ├── kraken.go
│   │       └── kucoin.go
│   │
│   ├── cache/
│   │   ├── cache.go         — PriceStorer interface
│   │   ├── memory.go        — in-memory реалізація (TTL 30s)
│   │   └── redis.go         — Redis реалізація
│   │
│   ├── scheduler/
│   │   └── scheduler.go     — background job manager
│   │
│   ├── validator/
│   │   └── validator.go     — обгортка go-playground/validator
│   │
│   └── ws/
│       ├── hub.go           — реєстр WebSocket з'єднань
│       ├── client.go        — ReadPump + WritePump goroutines
│       ├── server.go        — broadcast loop (2s)
│       └── handler.go       — Fiber WS upgrade
│
├── migrations/              — SQL файли міграцій
│   ├── 000001_initial_schema.{up,down}.sql
│   └── 000002_orders.{up,down}.sql
│
└── docs/                    — документація (цей каталог)
```

---

## Шари архітектури

### 1. Transport Layer (handlers/)
- Приймає HTTP запит
- Валідує вхідні дані через `validator.Validate()`
- Витягує `userID` з JWT через `middleware.GetUserID(c)`
- Викликає сервіс або репозиторій
- Повертає JSON відповідь

### 2. Service Layer (services/)
- **SyncService** — оркеструє паралельний синк: запускає goroutine на кожну біржу, збирає результати
- **PriceService** — зберігає ціни у кеші (memory або Redis), оновлює БД
- **EncryptionService** — AES-256-GCM для API ключів користувачів

### 3. Exchange Adapters (services/exchange/)
Два інтерфейси:
- **Exchange** — читання: баланси, позиції, закриті угоди, ціни
- **Trader** (розширення) — торгівля: PlaceOrder, CancelOrder, GetOrderStatus

Лише Binance, OKX, Bybit реалізують Trader. Gate.io, Kraken, KuCoin — тільки Exchange.

### 4. Repository Layer (models/)
Прямий SQL через `sqlx`. Без ORM. `INSERT IGNORE` / `ON DUPLICATE KEY UPDATE` для idempotency.

### 5. Cache Layer (cache/)
Інтерфейс `PriceStorer` дозволяє замінити in-memory кеш на Redis без зміни жодного сервісу.

---

## Паттерни

### Dependency Injection
Всі залежності передаються через конструктор. Глобальних змінних нема.

```go
// main.go
enc := services.NewEncryptionService(cfg.EncryptionKey)
priceService := services.NewPriceService(db, priceStore, logger)
orderHandler := handlers.NewOrderHandler(orderRepo, portfolioRepo, enc, logger)
```

### Optional Interface (Trader)
```go
ex, ok := h.exchanges[req.Exchange]      // Exchange завжди є
trader, ok := ex.(exchange.Trader)       // Trader — тільки у Binance/OKX/Bybit
if !ok {
    return c.Status(400).JSON(...)       // зрозуміла помилка замість паніки
}
```

### Graceful Shutdown
```
os.Signal → cancel context → зупиняє WS loop + scheduler → app.ShutdownWithTimeout(5s) → db.Close()
```

---

## База даних

**Engine:** MySQL / MariaDB (WAMP, порт 3306)
**БД:** `tradetracker_go`
**Pool:** max 25 з'єднань

Таблиці (детально в [migrations/](../migrations/)):
| Таблиця | Призначення |
|---|---|
| `users` | Акаунти користувачів |
| `assets` | Відомі активи (BTC, ETH...) |
| `user_portfolios` | Спотові баланси |
| `open_positions` | Відкриті ф'ючерсні позиції |
| `position_history` | Закриті угоди |
| `external_api_credentials` | Зашифровані API ключі бірж |
| `orders` | Торгові ордери (розміщені через API) |

---

## Конкурентність

- Кожна біржа в синку — окрема goroutine (`WaitGroup`)
- WS клієнт = 2 goroutines (ReadPump + WritePump)
- Scheduler — goroutine per job
- Shared state захищений `sync.RWMutex` (hub, memory cache)
- Redis операції ізольовані 200ms таймаутом

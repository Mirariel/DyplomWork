# TradeTracker Go — Швидкий старт

## Вимоги

**Локальний запуск:**
- Go 1.21+
- MySQL 8.0 (WAMP або окремо)
- Node.js 20+ (для frontend dev-сервера)

**Docker-запуск (рекомендовано):**
- Docker Desktop 4.x

---

## Варіант 1 — Docker (все одною командою)

```bash
git clone <repo>
cd tradetracker-go
cp .env.example .env   # заповнити JWT_SECRET і APP_KEY

docker compose up --build -d
```

Що підіймається автоматично:
| Сервіс | URL |
|---|---|
| Frontend (React/nginx) | http://localhost |
| Backend API (Go/Fiber) | http://localhost:8080 |
| Prometheus metrics | http://localhost:8080/metrics |
| MySQL | localhost:3306 |
| Redis | localhost:6379 |

Міграції застосовуються автоматично (one-shot `migrate` контейнер).

```bash
# Корисні команди
make docker-logs    # tail логів всіх сервісів
make docker-down    # зупинити
make docker-reset   # зупинити + видалити volumes (скидає БД)
```

---

## Варіант 2 — Локальний запуск

### 1. Клонування і залежності

```bash
git clone <repo>
cd tradetracker-go
go mod download
```

### 2. Конфігурація

```bash
cp .env.example .env
```

Заповнити `.env`:
```env
APP_PORT=8080
APP_DEBUG=true

JWT_SECRET=change_this_to_random_string_min_32_chars

DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=tradetracker

# AES-256-GCM ключ — рівно 32 символи!
APP_KEY=your-32-character-encryption-key!

# Redis (опціонально — без нього in-memory cache)
REDIS_URL=

# Telegram сповіщення (опціонально)
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=
```

### 3. База даних

```sql
-- В MySQL:
CREATE DATABASE tradetracker CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

```bash
make migrate-up
# або: go run ./cmd/migrate/... -cmd up
```

### 4. Запуск backend

```bash
go run ./cmd/server/...
```

Очікуваний вивід:
```
time=2026-06-21T10:00:00 level=INFO msg="Database connected"
time=2026-06-21T10:00:00 level=INFO msg="price cache: in-memory"
time=2026-06-21T10:00:00 level=INFO msg="server starting" port=8080
```

Перевірка:
```bash
curl http://localhost:8080/health
# {"status":"ok","version":"0.1.0"}
```

### 5. Frontend (dev-сервер)

```bash
cd frontend
npm install
npm run dev
# → http://localhost:5173
```

---

## Перший запит (API)

### 1. Зареєструватись
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","email":"alice@test.com","password":"secret123"}'
```

Збережіть `token` з відповіді.

### 2. Підключити API ключі біржі
```bash
curl -X POST http://localhost:8080/api/portfolio/credentials \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"exchange":"binance","api_key":"...","api_secret":"..."}'
```

### 3. Синхронізувати портфель
```bash
curl -X POST http://localhost:8080/api/sync/full \
  -H "Authorization: Bearer <token>"
```

### 4. Переглянути портфель
```bash
curl http://localhost:8080/api/portfolio/ \
  -H "Authorization: Bearer <token>"
```

---

## Тести

```bash
# Unit тести (без MySQL)
go test ./internal/services/exchange/...
go test ./internal/ws/...

# Інтеграційні тести репозиторіїв (потрібна MySQL tradetracker_test)
go test ./internal/models/...

# Всі тести
go test ./...
```

Інтеграційні тести автоматично створюють БД `tradetracker_test` і застосовують міграції.
Для нестандартного DSN: `TEST_DB_DSN=user:pass@tcp(host:port)/tradetracker_test?parseTime=true`

---

## Міграції

```bash
make migrate-up        # застосувати всі нові
make migrate-down      # відкотити останню
make migrate-version   # поточна версія (має бути 6)
make migrate-force     # примусово встановити версію (якщо dirty=true)
```

---

## Структура логів

**Dev режим** (`APP_DEBUG=true`) — human-readable:
```
time=2026-06-21T10:00:00 level=INFO msg="order: placed" user_id=1 exchange=binance symbol=BTC
```

**Prod режим** (`APP_DEBUG=false`) — JSON:
```json
{"time":"2026-06-21T10:00:00Z","level":"INFO","msg":"order: placed","user_id":1,"exchange":"binance"}
```

---

## Зупинка

`Ctrl+C` — graceful shutdown: закінчує поточні запити (до 5 секунд), закриває БД.

---

## Документація

| Файл | Зміст |
|---|---|
| `docs/architecture.md` | Архітектура, паттерни, структура коду |
| `docs/api-reference.md` | Всі HTTP та WebSocket ендпоінти |
| `docs/exchange-adapters.md` | Деталі інтеграції кожної біржі |
| `docs/trading-guide.md` | Ордери, боти, smart orders |
| `DescriptionsProject.md` | Живий стан проєкту (фази, roadmap) |
| `PROBLEMS.md` | Відомі проблеми і рішення |

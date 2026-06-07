# TradeTracker Go — Швидкий старт

## Вимоги

- Go 1.21+
- MySQL / MariaDB (WAMP або окремо)
- Git

---

## Встановлення

```bash
git clone <repo>
cd tradetracker-go

# Встановити залежності
go mod download
```

---

## Конфігурація

Скопіюйте `.env.example` у `.env` і заповніть:

```env
APP_PORT=8080
APP_DEBUG=true

JWT_SECRET=change_this_to_random_string_min_32_chars

DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=tradetracker_go

# AES-256-GCM ключ для шифрування API ключів бірж (32 hex символи = 16 bytes)
APP_KEY=b84260907e594d6c97a5a8f7d98305c4

# Опціонально: Redis для кешу цін (залишити порожнім для in-memory)
REDIS_URL=

# Опціонально: Claude API для AI-підказок
ANTHROPIC_API_KEY=
```

---

## Підготовка бази даних

1. Запустіть MySQL (WAMP → зелений значок)
2. Створіть БД:
   ```sql
   CREATE DATABASE tradetracker_go CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
   ```
3. Запустіть міграції:
   ```bash
   go run ./cmd/migrate/... up
   ```
   Або використайте Makefile (якщо є `make`):
   ```bash
   make migrate-up
   ```

---

## Запуск сервера

```bash
go run ./cmd/server/...
```

Очікуваний вивід:
```
time=2025-01-15T10:00:00 level=INFO msg="Database connected"
time=2025-01-15T10:00:00 level=INFO msg="price cache: in-memory"
time=2025-01-15T10:00:00 level=INFO msg="server starting" port=8080 ws=ws://localhost:8080/ws
```

Перевірка:
```bash
curl http://localhost:8080/health
# {"status":"ok","version":"0.1.0"}
```

---

## Перший запит

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

## Збірка бінарника

```bash
go build -o bin/server ./cmd/server/...
./bin/server
```

---

## Міграції

```bash
# Застосувати всі нові міграції
go run ./cmd/migrate/... up

# Відкотити останню
go run ./cmd/migrate/... down

# Поточна версія
go run ./cmd/migrate/... version

# Примусово встановити версію (якщо dirty=true)
go run ./cmd/migrate/... force 1
```

---

## Структура логів

**Dev режим** (`APP_DEBUG=true`) — human-readable:
```
time=2025-01-15T10:00:00 level=INFO msg="order: placed" user_id=1 exchange=binance symbol=BTC order_id=3847291
```

**Prod режим** (`APP_DEBUG=false`) — JSON:
```json
{"time":"2025-01-15T10:00:00Z","level":"INFO","msg":"order: placed","user_id":1,"exchange":"binance","symbol":"BTC","order_id":"3847291"}
```

---

## Зупинка сервера

`Ctrl+C` — graceful shutdown: сервер закінчить поточні запити (до 5 секунд) і закриє підключення до БД.

---

## Документація

| Файл | Зміст |
|---|---|
| `docs/architecture.md` | Архітектура, паттерни, структура коду |
| `docs/api-reference.md` | Всі HTTP та WebSocket ендпоінти |
| `docs/exchange-adapters.md` | Деталі інтеграції кожної біржі |
| `docs/trading-guide.md` | Розміщення ордерів, управління, помилки |
| `CLAUDE.md` | Живий стан проєкту для AI-асистента |
| `PROBLEMS.md` | Відомі проблеми і рішення |

# PROBLEMS.md — Проблеми та їх вирішення

Хронологічний журнал нетривіальних проблем, з якими ми зіткнулись під час розробки.
Читати перед початком роботи з новим компонентом — щоб не наступати на ті самі граблі.

---

## P-001 — Fiber несумісний з gorilla/websocket

**Коли:** Реалізація WebSocket сервера.

**Проблема:**
Спробували підключити `gorilla/websocket` напряму в Fiber handler:
```go
conn, err := upgrader.Upgrade(c.Context()...) // не компілюється
```
Fiber використовує `fasthttp` під капотом. `gorilla/websocket` очікує стандартний
`net/http.ResponseWriter` — ці інтерфейси несумісні, `Upgrade()` не працює.

**Рішення:**
Використати офіційний адаптер `github.com/gofiber/websocket/v2`, який побудований
поверх `fasthttp/websocket` (порт gorilla API для fasthttp).
```go
import fiberws "github.com/gofiber/websocket/v2"

app.Get("/ws", fiberws.New(func(c *fiberws.Conn) {
    client := &Client{conn: c.Conn, ...}  // c.Conn — *fasthttp/websocket.Conn
}))
```
`client.go` імпортує `github.com/fasthttp/websocket`, не `gorilla/websocket`.

**Урок:** При роботі з Fiber завжди перевіряти чи існує офіційний fiber-middleware
замість стандартного net/http пакету.

---

## P-002 — sqlx `SELECT *` падає якщо є зайва колонка в таблиці

**Коли:** Перший живий тест — реєстрація повертала "user already exists", логін — "invalid credentials".

**Проблема:**
```go
type User struct {
    ID        int64     `db:"id"`
    Username  string    `db:"username"`
    Email     string    `db:"email"`
    Password  string    `db:"password"`
    CreatedAt time.Time `db:"created_at"`
    // updated_at — ВІДСУТНЯ
}

r.db.Get(&u, "SELECT * FROM users WHERE email = ?", email)
// → error: missing destination name updated_at
```
`sqlx.Get` за замовчуванням — **не unsafe**. Якщо `SELECT *` повертає колонку,
якої немає в структурі — повертає помилку `missing destination name <column>`.
Ця помилка перехоплювалась і маскувалась під "user already exists" / "invalid credentials".

**Рішення:**
Додати **всі** колонки таблиці до структури:
```go
type User struct {
    ...
    UpdatedAt time.Time `db:"updated_at" json:"-"`
}
```

**Урок:** При `SELECT *` struct має покривати **всі** колонки таблиці.
Або використовувати `db.Unsafe()` (ризик), або явно перераховувати колонки в SELECT.
Завжди перевіряти struct проти реальної схеми після зміни міграції.

**Зачеплені файли:** `models/user.go`, `models/portfolio.go`

---

## P-003 — OpenPosition.Leverage int vs VARCHAR(10) в БД

**Коли:** Перевірка моделей перед першим тестом.

**Проблема:**
```go
type OpenPosition struct {
    Leverage int `db:"leverage"`  // БД: VARCHAR(10) — "10x"
}
```
MySQL не може відсканувати `"10x"` в `int` — runtime panic при `Select`.

**Рішення:**
```go
Leverage string `db:"leverage"`  // "10x", "5x", "1x"
```
У `sync_service.go` при збереженні:
```go
Leverage: fmt.Sprintf("%dx", p.Leverage),  // int → "10x"
```
У `ws/server.go` аналогічно через `formatLeverage(int) string`.

**Урок:** Leverage на біржах завжди приходить як рядок (`"10x"`).
Зберігати як `VARCHAR`, не намагатись парсити в число.

---

## P-004 — PositionHistory структура не відповідає схемі БД

**Коли:** Аналіз portfolio.go перед першим тестом.

**Проблема:** Структура `PositionHistory` в Go була написана "по пам’яті" і
мала кілька неправильних назв полів:

| Go поле | Go `db` тег | Реальна колонка в БД |
|---|---|---|
| `ClosePrice` | `close_price` | `exit_price` |
| `PnL` | `pnl` | `realized_pnl` |
| `AssetID` | `asset_id` | *(відсутнє в таблиці)* |

Також були відсутні: `MarginMode`, `Leverage`, `Fee`, `MaxSize`, `OpenedAt`, `CreatedAt`.

**Рішення:** Переписати структуру повністю, звіривши з `migrations/000001_initial_schema.up.sql`:
```go
type PositionHistory struct {
    RealizedPnl float64 `db:"realized_pnl"`
    ExitPrice   float64 `db:"exit_price"`
    // asset_id — не зберігаємо в history (немає в схемі)
    ...
}
```

**Урок:** Після написання міграції — одразу звіряти всі Go-структури з DDL.
Не покладатися на пам'ять. `PROBLEMS.md` існує саме для таких випадків.

---

## P-005 — LastInsertId повертає 0 для INT UNSIGNED в MySQL (WAMP)

**Коли:** Реєстрація — INSERT спрацьовував (юзер в БД), але відповідь "user already exists".

**Проблема:**
```go
res, err := r.db.Exec("INSERT INTO users ...")
id, _ := res.LastInsertId()  // повертає 0 на деяких конфігах MySQL з INT UNSIGNED
return r.FindByID(id)         // FindByID(0) → sql: no rows → error
```
На конфігурації WAMP MySQL 8.4 з `INT UNSIGNED AUTO_INCREMENT` — `LastInsertId()`
може повертати 0 замість реального ID. INSERT при цьому успішний.

**Рішення:**
Після INSERT шукати по унікальному полю (email) замість ID:
```go
_, err = r.db.Exec("INSERT INTO users (username, email, password) VALUES (?, ?, ?)", ...)
if err != nil { return nil, err }
return r.FindByEmail(email)  // надійніше ніж LastInsertId
```

**Урок:** Не покладатися на `LastInsertId()` з `INT UNSIGNED` в MySQL.
Надійніший підхід — `SELECT` по унікальному полю після INSERT,
або `INSERT ... RETURNING id` (MySQL 8.0+, але не всі драйвери підтримують).

---

## P-006 — Старий процес тримає порт 8080 після перезапуску

**Коли:** Кожного разу при перезапуску сервера під час розробки.

**Проблема:**
`go run ./cmd/server/... &` запускає процес у фоні.
`pkill -f "go run"` не вбиває скомпільований бінарник — вбиває лише `go run` wrapper.
Скомпільований процес продовжує тримати порт.

**Рішення:**
```bash
# Знайти PID по порту
netstat -ano | grep :8080 | grep LISTENING

# Вбити конкретний PID (Windows)
taskkill //F //PID <PID>
```

**Урок:** На Windows `pkill` не працює надійно для Go бінарників запущених через `go run`.
Використовувати `taskkill //F //PID` по конкретному PID з `netstat`.
В майбутньому — додати `make stop` в Makefile:
```makefile
stop:
    -taskkill //F //IM server.exe 2>nul || true
```

---

## P-007 — Неправильне маскування помилок в handlers

**Коли:** Діагностика P-002, P-005 — всі різні помилки виглядали однаково.

**Проблема:**
```go
user, err := h.users.Create(body.Username, body.Email, body.Password)
if err != nil {
    return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "user already exists"})
}
```
Будь-яка помилка з `Create` (sqlx mismatch, LastInsertId=0, мережева помилка)
маскувалась під "user already exists". Це унеможливило діагностику.

**Рішення (тимчасове):** Змінено щоб handler повертав реальну помилку в dev-режимі.
**Рішення (правильне — зробити в Фазі 1.2):**
```go
if err != nil {
    if isMySQLDuplicateError(err) {
        return c.Status(409).JSON(fiber.Map{"error": "email already registered"})
    }
    return c.Status(500).JSON(fiber.Map{"error": "internal error"})
}
```
```go
import "github.com/go-sql-driver/mysql"

func isMySQLDuplicateError(err error) bool {
    var mysqlErr *mysql.MySQLError
    return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
```

**Урок:** Ніколи не маскувати всі помилки під один HTTP код.
Завжди логувати реальну помилку (хоча б в dev), навіть якщо клієнту показуємо generic message.

---

## P-008 — `go build` кешує старий бінарник при `go run`

**Коли:** Під час відлагодження P-005 та P-007.

**Проблема:** Правили код, але сервер після перезапуску поводився як старий.
Причина: попередній скомпільований процес ще живий (P-006), і нова копія
запускалась на іншому порту або не запускалась взагалі через конфлікт порту.
Здавалось що зміни не застосовуються.

**Рішення:** Завжди перевіряти що старий процес вбитий (P-006) перед запуском нового.
Додати в Makefile:
```makefile
restart:
    -taskkill //F //IM server.exe 2>nul
    go run ./cmd/server/...
```

**Урок:** `go run` кожного разу компілює свіжий бінарник — якщо він не запускається,
значить старий процес ще живий. Спочатку перевіряємо порт, потім запускаємо.

---

## P-009 — Витяг JWT токена з JSON в bash (Windows)

**Коли:** Автоматичні тести в bash під час розробки.

**Проблема:** Стандартні bash-інструменти для парсингу JSON (`grep -o`, `sed`, `python3`)
або не встановлені в WAMP bash, або дають непередбачувані результати на Windows.
```bash
TOKEN=$(echo $RESP | sed 's/.*"token":"\([^"]*\)".*/\1/')
# → повертає весь рядок або порожній рядок
```

**Рішення:** Використовувати `curl` з cookie-based auth замість Bearer токена:
```bash
# Зберігаємо cookie в файл
curl -c /tmp/cookies.txt -X POST http://localhost:8080/api/auth/login ...

# Використовуємо cookie у наступних запитах
curl -b /tmp/cookies.txt http://localhost:8080/api/auth/me
```
Або встановити `jq` для Windows і використовувати: `echo $RESP | jq -r '.token'`

**Урок:** На Windows WAMP bash — використовувати cookie auth для тестування,
або встановити `jq`. Bearer auth тестувати через Postman/Bruno/curl з `jq`.

---

## P-010 — `gofiber/limiter` вимагає окремого `go get` попри те що входить до fiber/v2

**Коли:** Додавання rate limiting (Фаза 1.1).

**Проблема:**
```go
import "github.com/gofiber/fiber/v2/middleware/limiter"
```
Після додавання імпорту `go build ./...` падав з помилкою:
```
missing go.sum entry for module providing package github.com/tinylib/msgp/msgp
(imported by github.com/gofiber/fiber/v2/middleware/limiter)
```
`limiter` middleware зберігає стан в пам'яті через `msgp` (MessagePack serialization).
Цей транзитивний dep не потрапив до `go.sum` автоматично, бо раніше limiter
ніколи не імпортувався в проекті.

**Рішення:**
```bash
go get github.com/gofiber/fiber/v2/middleware/limiter@v2.52.12
```
Команда дозавантажила `tinylib/msgp` і `philhofer/fwd` та оновила `go.sum`.

**Урок:** При першому використанні будь-якого fiber middleware — запускати
`go get <package>@<version>` навіть якщо fiber вже є в `go.mod`.
Fiber middleware можуть мати транзитивні залежності яких ще немає в `go.sum`.

---

## P-011 — Kraken API secret — base64-encoded, не raw string

**Коли:** Реалізація Kraken адаптера (Фаза 2).

**Проблема:**
Більшість бірж (Binance, Bybit, Gate.io) очікують API secret як raw string для HMAC.
Kraken — єдина з реалізованих бірж де API secret зберігається в base64 і **декодується** перед використан
ням:
```go
// Неправильно (як в інших біржах):
mac := hmac.New(sha512.New, []byte(creds.APISecret))

// Правильно для Kraken:
secretBytes, _ := base64.StdEncoding.DecodeString(creds.APISecret)
mac := hmac.New(sha512.New, secretBytes)
```
Також підпис Kraken має унікальну структуру: `path + SHA256(nonce + postData)` а не просто `timestamp + params`.

**Рішення:**
Реалізувати `krakenSign()` з explicit base64 decode в `kraken.go`.
Не намагатись повторно використовувати `hmacSHA256/512` з `helpers.go`.

**Урок:** Перед реалізацією адаптера **читати офіційну документацію по підпису**, не припускати 
що схема як у Binance.
Kraken: Base64(HMAC-SHA512(path+SHA256(nonce+data), Base64Decode(secret))).

---

## P-012 — Помилка "could not determine executable to run" при `npx tailwindcss init`

**Коли:** Ініціалізація Tailwind CSS у новій папці `frontend`.

**Проблема:**
При спробі виконати `npx tailwindcss init -p` у середовищі WAMP/Windows, npm видавав помилку, що не може знайти виконуваний файл, навіть після успішного `npm install`.

**Рішення:**
Файли конфігурації (`tailwind.config.js` та `postcss.config.js`) були створені вручну з відповідним вмістом. Це швидше, ніж відлагоджувати специфічні проблеми шляхів npm у Windows.

**Урок:** Якщо стандартні CLI-інструменти ініціалізації дають збій через оточення, простіше створити базові конфіги вручну за шаблоном.

---

## P-013 — Interactive prompt при `npm create vite`

**Коли:** Створення фронтенд-проекту.

**Проблема:**
Команда `npm create vite@latest frontend -- --template react-ts` все одно видавала інтерактивний запит "Install with npm and start now?", що блокувало автоматичне виконання.

**Рішення:**
Використано прапорець `--no-interactive` (або запуск через `npx -y` з чітким вказанням параметрів).

**Урок:** Для повної неінтерактивності в сучасних версіях Vite CLI потрібно додавати `--no-interactive`.

---

## P-014 — `getUserCreds` дублюється між `OrderHandler` і `SmartOrderService`

**Коли:** Реалізація SmartOrderService (Фаза 4).

**Проблема:**
Логіка розшифрування credentials (GetCredentials → знайти по exchange → Decrypt key/secret/passphrase)
вже була написана в `handlers/order.go` як приватний метод `getUserCreds`.
При реалізації `SmartOrderService` довелось продублювати ту саму логіку в сервісному шарі,
бо handler-метод недоступний за межами пакету.

**Рішення (тимчасове):** Дублювання залишено — код коректний, просто не DRY.

**Рішення (правильне — рефакторинг):**
Винести `getUserCreds` в окремий хелпер або метод `PortfolioRepository`:
```go
// internal/services/credentials.go
func ResolveCredentials(portfolio *models.PortfolioRepository, enc *EncryptionService,
    userID int64, exchangeName string) (exchange.Credentials, error) { ... }
```
Тоді і handler, і сервіс використовують одну функцію.

**Урок:** Логіка що потрібна і в handlers, і в services — належить до services або shared helpers,
не до handler-методів.

**Зачеплені файли:** `handlers/order.go`, `services/smart_order_service.go`

---

## P-015 — `ErrNoCredentials` не існував в пакеті exchange

**Коли:** Реалізація `SmartOrderService.getUserCreds` (Фаза 4).

**Проблема:**
`SmartOrderService.getUserCreds` повертає помилку коли credentials не знайдено.
`handlers/order.go` використовував `fiber.ErrNotFound` як sentinel error — але це fiber-специфічна
помилка яку некоректно використовувати в сервісному шарі (порушує залежності: service → http framework).

**Рішення:**
Додано `var ErrNoCredentials = errors.New("no active credentials found for exchange")`
в `internal/services/exchange/interface.go` — нейтральне місце без зовнішніх залежностей.
`SmartOrderService` повертає `exchange.ErrNoCredentials`, handler залишив `fiber.ErrNotFound` (прийнятно
бо він у http-шарі).

**Урок:** Sentinel errors для доменної логіки визначати в exchange/domain пакеті,
не брати готові з HTTP-фреймворків.

**Зачеплені файли:** `services/exchange/interface.go`, `services/smart_order_service.go`

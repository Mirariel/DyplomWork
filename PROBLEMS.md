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

---

## P-016 — `GetLivePrices(nil)` завжди повертав порожній map

**Коли:** Аудит коду після Фази 4.

**Проблема:**
`ws/server.go` викликав `s.prices.GetLivePrices(nil)` кожні 2 секунди для broadcast.
Але `GetLivePrices` ітерував по `symbols` — якщо nil, цикл не виконувався і повертався порожній map.
Всі WS-клієнти отримували `"spot_prices": {}` — порожній об'єкт.

```go
// price_service.go — стара логіка
result := make(map[string]float64, len(symbols))
for _, sym := range symbols { // nil → 0 ітерацій
    ...
}
return result // {} завжди
```

**Рішення:**
В `GetLivePrices`: якщо `len(symbols) == 0` — повертати всі доступні ціни з кешу:
```go
if len(symbols) == 0 {
    result := make(map[string]float64)
    for _, prices := range allPrices {
        for sym, price := range prices {
            if price > 0 && !stables[sym] { result[sym] = price }
        }
    }
    return result
}
```

**Урок:** API де nil і [] мають різну семантику — задокументувати явно в коментарі до функції.
Бажано мати тест на цей сценарій.

**Зачеплені файли:** `services/price_service.go`, `ws/server.go`

---

## P-017 — `roundFloat()` давав невірні результати для від'ємних чисел

**Коли:** Аудит коду після Фази 4.

**Проблема:**
```go
// Стара реалізація в ws/server.go
func roundFloat(v float64, decimals int) float64 {
    pow := 1.0
    for range decimals { pow *= 10 }
    return float64(int(v*pow+0.5)) / pow  // BUG!
}
```
Для від'ємного `v = -5.555`, decimals=2:
`int(-5.555*100 + 0.5)` = `int(-555.0)` = `-555` → результат `-5.55`
Очікувалось `-5.56`.

Усі від'ємні PnL% (збиткові позиції) відображались з похибкою округлення.

**Рішення:**
```go
func roundFloat(v float64, decimals int) float64 {
    pow := math.Pow(10, float64(decimals))
    return math.Round(v*pow) / pow
}
```
`math.Round` коректно обробляє від'ємні числа (round half away from zero).

**Урок:** Ніколи не використовувати `int(v + 0.5)` для округлення — це класична пастка.
Завжди `math.Round()`.

**Зачеплені файли:** `ws/server.go`

---

## P-018 — `stmt.Exec` помилки ігнорувались в `UpdatePrices`

**Коли:** Аудит коду після Фази 4.

**Проблема:**
```go
// price_service.go
stmt.Exec(1.0, sym)     // повертає (sql.Result, error) — error ігнорується
stmt.Exec(price, sym)   // те саме
```
Якщо оновлення ціни в БД провалювалось (з'єднання, таймаут, дедлок) — помилка
ковталась мовчки. Стейлі дані в БД, ніякого лога.

**Рішення:**
```go
if _, err := stmt.Exec(price, sym); err != nil {
    ps.logger.Error("prices: update failed", "symbol", sym, "error", err)
}
```

**Урок:** Ніколи не ігнорувати повернені помилки з `Exec`. Навіть якщо не хочемо зупиняти
цикл — мінімум логуємо.

**Зачеплені файли:** `services/price_service.go`

---

## P-019 — Невідповідність типів між WS LivePosition і DB Position

**Коли:** Реалізація React frontend (Фаза 7).

**Проблема:**
WS сервер надсилає `LivePosition` (поля: `entry_price`, `pnl`, `pnl_pct`, немає `id`),
а frontend спочатку типізував WS дані як DB `Position` (поля: `avg_price`, `unrealized_pnl`, `id`).
Також WS повідомлення мало `type: "update"` з полями `positions` і `spot_prices`,
але хук очікував `type: "positions"` і `type: "prices"` зі вкладеним `.data`.

**Рішення:**
1. Додали окремий `LivePosition` інтерфейс в `api.ts` з полями WS сервера.
2. Оновили `ws.ts` щоб повертати `LivePosition[]` замість `Position[]`.
3. Виправили парсинг повідомлень: `type === 'update'` → читати `msg.positions` і `msg.spot_prices`.

**Урок:** WS і REST API повертають різні структури для "позицій" — WS оптимізований для live дисплею
(розрахований PnL, entry_price), REST — для CRUD (avg_price, unreалізований PnL з БД).

**Зачеплені файли:** `frontend/src/ws.ts`, `frontend/src/api.ts`, `frontend/src/pages/Dashboard.tsx`

---

## P-020 — Невірні URL ендпоінтів в api.ts

**Коли:** Реалізація React frontend (Фаза 7).

**Проблема:**
Кілька URL в API-клієнті не збігались з маршрутами backend:
- `cancelOrder` → POST `/orders/:id/cancel` (мало бути DELETE `/orders/:id`)
- `cancelSmartOrder` → POST `/smart-orders/:id/cancel` (мало бути DELETE `/smart-orders/:id`)
- `takeSnapshot` → POST `/analytics/snapshots` (мало бути POST `/analytics/snapshot`)
- `updatePositionComment` → `/portfolio/positions/:id/comment` (мало бути `/positions/:id/comment`)
- `syncHistory` відправляв `{ days }` в body (мало бути query param)
- `updatePrices` → POST (мало бути GET `/sync/prices`)

**Рішення:** Виправили кожен URL/метод відповідно до маршрутів в `main.go`.

**Урок:** Завжди звіряти URL фронту з `main.go` — вони можуть розходитись після рефакторингу.

**Зачеплені файли:** `frontend/src/api.ts`

---

## P-021 — `node_modules` і `dist` потрапили в git

**Коли:** Перший коміт frontend (Фаза 7).

**Проблема:**
`frontend/` не мала `.gitignore`. Команда `git add frontend/` рекурсивно підхопила
`node_modules/` (~10 000 файлів) і `dist/` у коміт. Репозиторій роздувся на ~1.5M рядків.

**Рішення:**
```bash
# Видалити з індексу без видалення з диску
git rm --cached -r frontend/node_modules frontend/dist

# Додати в .gitignore
echo "frontend/node_modules/" >> .gitignore
echo "frontend/dist/" >> .gitignore
```
Потім окремий коміт для прибирання.

**Урок:** Перед `git add <нова директорія>` — переконатись що в проекті або кореневому `.gitignore`
є рядки для `node_modules/` і build-директорій.

**Зачеплені файли:** `.gitignore`

---

## P-022 — `vite-env.d.ts` відсутній — TS не знає тип CSS-імпортів

**Коли:** `npm run build` в Фазі 7.

**Проблема:**
```
src/main.tsx(5,8): error TS2307: Cannot find module './index.css'
```
Vite розуміє `import './index.css'`, але TypeScript — ні. Для цього потрібен
файл `src/vite-env.d.ts` з `/// <reference types="vite/client" />`, який
оголошує CSS/SVG/PNG модулі як `string`.

**Рішення:** Створити `src/vite-env.d.ts`:
```typescript
/// <reference types="vite/client" />
```

**Урок:** Будь-який Vite + TypeScript проект потребує `vite-env.d.ts`. Без нього
tsc не знає як типізувати нестандартні імпорти (css, svg, env змінні).

**Зачеплені файли:** `frontend/src/vite-env.d.ts` (новий)

---

## P-023 — `onSave` в `EditableComment` — несумісні типи Promise

**Коли:** `npm run build` в Фазі 7.

**Проблема:**
```
src/pages/Portfolio.tsx(154,21): error TS2322:
Type 'Promise<Position>' is not assignable to type 'Promise<void>'.
```
`EditableComment` очікував `onSave: (v: string) => Promise<void>`,
але `mutateAsync` повертає `Promise<Position>` — TypeScript відмовляється
неявно відкидати значення.

**Рішення:** Явно відкинути значення через `.then(() => undefined)`:
```tsx
onSave={(comment) =>
  commentMutation.mutateAsync({ id: p.id, comment }).then(() => undefined)
}
```

**Урок:** `Promise<T>` не є підтипом `Promise<void>` в TypeScript strict mode.
Якщо інтерфейс оголошує `void`, треба або змінити тип пропу на `Promise<unknown>`,
або явно конвертувати через `.then(() => undefined)`.

**Зачеплені файли:** `frontend/src/pages/Portfolio.tsx`

---

## P-024 — React `key` prop з індексом масиву в динамічних таблицях

**Коли:** Аудит коду після запуску frontend (Фаза 7).

**Проблема:**
В `Analytics.tsx` рядки CoinPerformance і Arbitrage таблиць мали `key={i}` (індекс).
При refetch (сортування змінилось, нові дані) React не може правильно відстежити
ідентичність рядків → зайвий re-render, можливі баги з анімаціями/фокусом.

**Рішення:**
```tsx
// Було:
sorted.map((c, i) => <tr key={i}>

// Стало:
sorted.map((c) => <tr key={`${c.symbol}-${c.exchange}`}>

// Arbitrage:
opps.map((opp) => <tr key={`${opp.symbol}-${opp.buy_exchange}-${opp.sell_exchange}`}>
```

**Урок:** `key={index}` — завжди антипатерн для списків, які можуть оновлюватись або
пересортовуватись. Використовувати унікальні бізнес-ідентифікатори.

**Зачеплені файли:** `frontend/src/pages/Analytics.tsx`

---

## P-025 — `getUserCreds` продубльовано в 4 місцях

**Коли:** Аудит коду після завершення Фази 9.

**Проблема:**
Метод `getUserCreds(userID int64, exchangeName string)` з однаковою логікою
(GetCredentials → decrypt → return Credentials) існував у чотирьох місцях:
- `handlers/order.go` як `(h *OrderHandler) getUserCreds`
- `services/smart_order_service.go` як `(s *SmartOrderService) getUserCreds`
- `services/bot_service.go` як `(s *BotService) getUserCreds`
- `services/dca_service.go` як `(s *DCAService) getUserCreds`

Будь-яка зміна логіки (нова помилка, новий поле credentials) потребувала правки у 4 місцях.

**Рішення:**
Виділено пакетну функцію `services.GetUserCreds(portfolio, enc, userID, exchange)` у новий файл
`internal/services/creds.go`. Всі чотири місця замінено на один виклик цієї функції.

**Урок:** Якщо однаковий код з'являється в 3+ місцях — виділяти в shared helper.
Особливо якщо логіка містить криптографію або IO (де помилки дорогі).

**Зачеплені файли:** `internal/services/creds.go` (новий), `handlers/order.go`,
`services/smart_order_service.go`, `services/bot_service.go`, `services/dca_service.go`

---

## P-026 — git rebase конфліктує з `frontend/node_modules`

**Коли:** Перезапис повідомлень комітів через `git rebase -i` (кілька разів).

**Проблема:**
При спробі `git rebase -i` від коміту що передує "Add React + Vite frontend dashboard",
rebase зупинявся на кроці застосування цього коміту з помилкою:
```
error: The following untracked working tree files would be overwritten by merge:
    frontend/node_modules/.bin/vite
    frontend/node_modules/.bin/esbuild
    ...10000+ файлів...
Aborting
```
`frontend/node_modules/` присутній на диску (після попереднього `npm install`), але
не в git (є в `.gitignore`). Git не може застосувати коміт, що додає `frontend/`,
бо робочий каталог "забруднений" untracked файлами у тій директорії.

**Рішення:**
Перед кожним rebase тимчасово видаляти `frontend/node_modules` і `frontend/dist`:
```bash
rm -rf frontend/node_modules frontend/dist
# ... rebase ...
cd frontend && npm install   # відновити після rebase
```

**Урок:** Перед `git rebase -i` що зачіпає коміти з великими untracked директоріями —
видалити їх з диску. Після rebase відновити через менеджер пакетів.

**Зачеплені файли:** `.gitignore`, `frontend/`

---

## P-027 — Timezone mismatch в інтеграційних тестах DCABotRepository

**Коли:** Написання інтеграційних тестів (Фаза 11).

**Проблема:**
Тест `TestDCABotRepository_ListDue` використовував `time.Now().Add(±1*time.Hour)` щоб
розрізнити "прострочений" і "майбутній" `next_buy_at`. Тест стабільно падав:
```
dca_bot_repo_test.go:106: ListDue: got 2, want 1
```
MySQL `NOW()` і Go `time.Now()` могли розрізнятись через:
- різні налаштування timezone сесії MySQL (UTC vs локальна UTC+2)
- відмінність між `loc=UTC` (дефолт MySQL Go driver) і локальним часом Go

Бот з `next_buy_at = time.Now().Add(+1h)` потрапляв у `next_buy_at <= NOW()` через
timezone offset між Go і MySQL сервером.

**Рішення:**
Збільшити offset до ±48 годин — гарантовано більше за будь-яку можливу timezone різницю:
```go
due    := newTestDCABot(user.ID, time.Now().Add(-48*time.Hour))
notDue := newTestDCABot(user.ID, time.Now().Add(+48*time.Hour))
```

**Урок:** В інтеграційних тестах з time-based фільтрами завжди використовувати великі
offset-и (≥24h), щоб нівелювати timezone skew між Go runtime і DB сервером.

**Зачеплені файли:** `internal/models/dca_bot_repo_test.go`

---

## P-028 — MySQL `DATE` з `parseTime=true` сканується як повний datetime рядок

**Коли:** Інтеграційні тести SnapshotRepository (Фаза 11).

**Проблема:**
`PortfolioSnapshot.SnapshotDate` задекларований як `string` з тегом `db:"snapshot_date"`.
Очікувалось що MySQL `DATE` колонка поверне `"2026-06-01"`, але при `parseTime=true` в DSN
MySQL Go driver парсить `DATE` як `time.Time`. Коли `sqlx` сканує `time.Time` у поле `string`,
він викликає `.String()` що повертає повний RFC3339 формат:
```
got "2026-06-01T00:00:00Z", want "2026-06-01"
```

**Рішення:**
У тесті порівнювати через `strings.HasPrefix(date, "2026-06-01")` замість точного порівняння.
Це охоплює обидва формати незалежно від конфігурації MySQL Driver.

**Альтернатива (якщо критично):** Змінити тип поля на `time.Time` і форматувати в handler,
або додати `?parseTime=false` у DSN (ламає парсинг TIMESTAMP/DATETIME).

**Урок:** З `parseTime=true` MySQL повертає всі date-based типи як `time.Time`.
Не покладатись на те що `DATE` → `string` без перетворення.

**Зачеплені файли:** `internal/models/snapshot_repo_test.go`

---

## P-029 — Docker Desktop daemon не стартує з bash-сесії Claude Code

**Коли:** Запуск `docker compose up` після встановлення Docker Desktop (Фаза 12).

**Проблема:**
Docker Desktop 4.78.0 встановлено через `winget`, але `docker ps` повертає:
```
failed to connect to the docker API at npipe:////./pipe/docker_engine;
open //./pipe/docker_engine: The system cannot find the file specified.
```
Навіть після `start "" "Docker Desktop.exe"` з bash — pipe `docker_engine` не з'являється.
Причина: Docker Desktop на Windows потребує запуску через GUI (клік по ярлику або системний трей)
і проходження першого launch wizard (EULA, WSL2 setup, engine initialization).
Bash-сесія не має прав до Windows message loop / named pipe initialization.

**Рішення:**
1. Запустити Docker Desktop вручну (Start → Docker Desktop, або ярлик на робочому столі)
2. Почекати доки в системному треї з'явиться іконка кита зі статусом "Docker Desktop is running"
3. Після цього всі CLI команди (`docker`, `docker compose`) доступні з будь-якого термінала

**Урок:** Docker Desktop на Windows = GUI-застосунок. Daemon (dockerd) запускається лише через нього.
Не намагатись запускати через `start cmd.exe` — це не дає потрібного контексту.
При наступному сеансі переконатись що Docker Desktop вже запущений перед запуском `docker compose up`.

---

## P-030 — Blank screen після логіну: `null` замість `[]` з БД + nginx проксі на неіснуючий сервіс

**Коли:** Тестування після мікросервісного рефакторингу (Фаза 15).

**Проблема (два незалежних баги):**

### 1. `sqlx.Select` → `null` у JSON для нового користувача

`sqlx.Select` не ініціалізує slice коли результат порожній — змінна залишається `nil`.
Go серіалізує `nil` slice як `null` у JSON (не `[]`):

```go
var snaps []PortfolioSnapshot          // nil якщо немає рядків
r.db.Select(&snaps, "SELECT ...")
return c.JSON(snaps)                   // → "null"
```

На фронтенді React Query отримує `data = null` (не `undefined`), тому дефолт `= []`
у деструктуризації не спрацьовує:

```tsx
const { data: snapshots = [] } = useQuery(...)  // data = null, не undefined!
const chartData = snapshots.map(...)             // TypeError: null.map is not a function
```

Оскільки ErrorBoundary відсутній — crash рендеру = порожній білий екран.
Вражало: Dashboard, Portfolio, Orders, Bots, DCA, Smart Orders, Analytics —
будь-яка сторінка де є список і користувач щойно зареєстрований.

### 2. nginx проксі на `api:8080` (старе ім'я) після рефакторингу

Після перейменування сервісу `api` → `api-gateway` у `docker-compose.yml`
`nginx.conf` залишився з:
```nginx
proxy_pass http://api:8080;   # 502 Bad Gateway — сервіс не існує
```
Це блокувало всі API-запити у Docker-режимі (`localhost:3000`).

**Рішення:**

### 1. Ініціалізувати slice через `make` перед `Select`

```go
// Було:
var snaps []PortfolioSnapshot

// Стало:
snaps := make([]PortfolioSnapshot, 0)
```

Виправлено у: `snapshot.go`, `bot.go`, `dca_bot.go`, `order.go`,
`smart_order.go`, `portfolio.go`, `analytics_service.go`.

### 2. Оновити nginx.conf

```nginx
# Було:
proxy_pass http://api:8080;

# Стало:
proxy_pass http://api-gateway:8080;
```

**Урок:**
- У Go `nil` slice і порожній slice (`[]T{}`) — різні речі для `encoding/json`.
  Завжди ініціалізувати через `make([]T, 0)` у репозиторіях що повертають списки.
- При перейменуванні Docker-сервісу — шукати всі місця де згадується стара назва
  (`nginx.conf`, `.env.example`, `README`, health-check скрипти).
- Після рефакторингу архітектури — тестувати з **новим** (порожнім) користувачем,
  бо баги з `null`-відповідями проявляються лише при відсутності даних у БД.

**Зачеплені файли:** `frontend/nginx.conf`, `internal/models/*.go`, `internal/services/analytics_service.go`

---

## P-031 — Blank screen після додавання API ключа: `json:"-"` + React StrictMode подвійний виклик

**Коли:** Тестування додавання API credentials через UI після Фази 15.

**Проблема (два незалежних баги):**

### 1. `maskKey(c.api_key)` — TypeError на полі з тегом `json:"-"`

Portfolio сторінка відображала API ключі через функцію `maskKey`:

```tsx
// Portfolio.tsx — стара логіка
const maskKey = (key: string) =>
  key.length > 8 ? key.slice(0, 4) + '••••' + key.slice(-4) : '••••••••'

// Використання:
<p>{maskKey(c.api_key)}</p>      // c.api_key = undefined!
```

Поле `api_key_encrypted` в Go моделі має тег `json:"-"`:
```go
ApiKeyEncrypted string `db:"api_key_encrypted" json:"-"`
```
Тобто воно **ніколи не надсилається** в HTTP-відповіді. Але `Credential` інтерфейс в `api.ts`
не містив поля `api_key` взагалі — TypeScript це не перевіряв (обʼєкт прийшов як `any` з axios).
При виклику `maskKey(undefined)` → `undefined.length` → `TypeError` → crash рендеру → білий екран.

**Рішення:**
Додати `label` (VARCHAR 100) і `api_key_hint` (VARCHAR 20) колонки через міграцію 000007.
Сервер рахує hint на стороні бекенду (перші 4 + `••••` + останні 4 символи ключа) і зберігає в БД.
Frontend отримує готовий `api_key_hint` рядок — жодних обчислень на клієнті:

```go
// handlers/portfolio.go
hint := body.APIKey
if len(hint) > 8 {
    hint = hint[:4] + "••••" + hint[len(hint)-4:]
}
cred := &models.ExternalApiCredential{
    Label:      body.Label,
    ApiKeyHint: hint,
    ...
}
```

```tsx
// Portfolio.tsx — нова логіка
<p className="font-mono text-xs">{c.api_key_hint || '••••••••'}</p>
```

### 2. React StrictMode + `skipNextMeCall` ref — `me()` викликається двічі

`AuthContext.tsx` використовував `useRef` флаг `skipNextMeCall` щоб уникнути повторного виклику
`me()` після логіну (бо `setToken` в `login()` тригерить `useEffect([token])`):

```tsx
// AuthContext.tsx — стара логіка
const skipNextMeCall = useRef(false)
useEffect(() => {
  if (skipNextMeCall.current) {
    skipNextMeCall.current = false
    return
  }
  me().then(setUser).catch(() => { ... })
}, [token])

const login = async (...) => {
  ...
  skipNextMeCall.current = true  // 1-й запуск скіпається...
  setToken(data.token)           // ...але StrictMode запускає effect ДВА рази
}
```

React 18 StrictMode у dev-режимі навмисно монтує компоненти двічі (mount → unmount → mount).
При першому mount effect спрацьовує → `skipNextMeCall.current = false` (скидає флаг).
При другому mount effect знов спрацьовує → флаг вже `false` → `me()` викликається → 401 → logout.

**Рішення:**
Прибрати `skipNextMeCall` взагалі. Використати стандартний cleanup pattern з `cancelled` флагом:

```tsx
useEffect(() => {
  if (!token) { setLoading(false); return }
  let cancelled = false
  me()
    .then((u) => { if (!cancelled) setUser(u) })
    .catch(() => {
      if (cancelled) return
      localStorage.removeItem('tt_token')
      setToken(null); setUser(null)
    })
    .finally(() => { if (!cancelled) setLoading(false) })
  return () => { cancelled = true }
}, [token])
```

При StrictMode double-invoke: перший effect отримує `cancelled = true` через cleanup → ігнорує результат.
Другий effect виконується нормально. Флаг не потрібен.

**Урок:**
- `json:"-"` в Go struct означає "ніколи не в JSON". Якщо фронтенд очікує поле — воно **ніколи** не прийде.
  Масковані дані (key hints) рахувати на бекенді, не на клієнті.
- React StrictMode умисно ламає `useRef`-флаги між ефектами — вони не persist між double-invoke.
  Для async cleanup у `useEffect` — завжди `let cancelled = false` + `return () => { cancelled = true }`.
- Після `json:"-"` таг: TypeScript `interface` повинен відображати що реально приходить з API,
  не що є в Go struct.

**Зачеплені файли:**
`migrations/000007_credentials_label.up.sql` (новий),
`internal/models/portfolio.go` (ExternalApiCredential + UpsertCredential),
`internal/handlers/portfolio.go` (hint + label),
`frontend/src/api.ts` (Credential interface),
`frontend/src/pages/Portfolio.tsx` (maskKey видалено → api_key_hint),
`frontend/src/context/AuthContext.tsx` (skipNextMeCall → cancelled flag)

---

## P-032 — TradingView widget не рендериться в React SPA

**Коли:** Реалізація price chart у Portfolio DetailModal та CoinModal.

**Проблема:**
TradingView Lightweight Charts або Advanced Chart widget завантажуються через `<script>` тег
який ін'єктується динамічно в DOM через `useEffect`. Всередині цього скрипта виконується
`document.currentScript` — але для **динамічно ін'єктованих** скриптів цей атрибут завжди `null`.
В результаті widget не може визначити свій контейнер і не рендериться, без жодної помилки в консолі.

```js
// useEffect — inject script
const script = document.createElement('script')
script.src = 'https://s3.tradingview.com/tv.js'
script.onload = () => new TradingView.widget({ container_id: 'tv-chart', ... })
// ↑ widget внутрішньо робить: const me = document.currentScript → null → crash
```

**Рішення:**
Відмовились від TradingView embed. Замінили на власний компонент `PriceChart.tsx` —
`recharts` AreaChart + публічний Binance klines API:
```
GET https://api.binance.com/api/v3/klines?symbol=BTCUSDT&interval=1h&limit=100
```
- Не потребує ключів (публічний endpoint)
- Зелений/червоний градієнт залежно від динаміки
- Interval selector: 15m / 1h / 4h / 1d
- Props: `symbol`, `height`, `showIntervalSelector`, `defaultInterval`

**Урок:**
TradingView widget не підходить для React SPA якщо використовується через динамічний `<script>`.
`document.currentScript` = `null` для будь-якого скрипта, завантаженого після `DOMContentLoaded`.
Альтернативи: `lightweight-charts` (npm пакет, не через `<script>`), або власний компонент на recharts.

**Зачеплені файли:**
`frontend/src/components/PriceChart.tsx` (новий),
`frontend/src/components/CoinModal.tsx` (TVChart → PriceChart),
`frontend/src/pages/Portfolio.tsx` (TradingViewChart → PriceChart в DetailModal)

---

## P-033 — PnL% формула невірна для inverse (coin-margined) контрактів

**Коли:** WS broadcast розраховує live PnL% для відкритих ф'ючерсних позицій.

**Проблема:**
Початкова формула розрахунку PnL%:
```go
margin := p.EntryPrice * p.Quantity / float64(max(p.Leverage, 1))
pnlPct = p.PnL / margin * 100
```
Вона правильна **тільки** для linear (USDT-M) контрактів, де `Quantity` — кількість базової монети.

Для **inverse** (coin-margined) контрактів (наприклад BTCUSD на Bybit/OKX) `Quantity` — кількість
**контрактів** (кожен = $100), а не BTC. Тому чисельник `p.PnL` (у BTC) і знаменник `entryPrice × qty`
(у доларах × контракти = розмірність не збігається) дають повну нісенітницю.

**Рішення:**
Price-based формула — не залежить від розміру або валюти контракту:
```go
// PnL% = (markPrice − entryPrice) / entryPrice × leverage × 100
priceChange := (p.MarkPrice - p.EntryPrice) / p.EntryPrice
if strings.EqualFold(p.Side, "short") {
    priceChange = -priceChange
}
lev := p.Leverage
if lev < 1 { lev = 1 }
pnlPct = priceChange * float64(lev) * 100
```
Ця формула правильна і для linear, і для inverse, і для spot (lev=1).

**Урок:**
Для розрахунку PnL% ніколи не використовувати `pnl / margin`, якщо `Quantity` може бути
в контрактах (integers × номінал), а не в базовій монеті.
Завжди використовувати price-based: `(mark − entry) / entry × lev`.

**Зачеплені файли:**
`internal/ws/server.go` (broadcastToUser → pnlPct calculation)

---

## P-034 — OKX `lever` завжди "0" для cross-margin positions-history

**Коли:** Реалізація відображення закритих угод (Фаза 16.7).

**Проблема:**
OKX `/api/v5/account/positions-history` повертає поле `lever: "0"` для **всіх** cross-margin позицій у historical mode. Це не баг — це задокументована поведінка API: для cross-margin режиму leverage є динамічним і не прив'язаний до конкретної позиції.

```go
// Результат: lever = "0" для кожної cross-margin угоди
notionalUsd lever  mgnMode
"125.4"     "0"    "cross"
"259.95"    "0"    "cross"
```

У нас початково зберігалось `"0x"` → UI показував порожнє плече або `"0x"` для всіх OKX угод.

**Рішення:**
Два кроки:
1. Для leverage — fallback на `/api/v5/account/leverage-info?instId={}&mgnMode=cross` після отримання positions-history. Повертає **поточне** налаштування плеча для інструменту (не histórical, але краще ніж 0).
2. В UI — якщо `lever = "0x"` (невідоме) → показувати `"Cross"` замість нічого, бо це cross-margin режим.

```go
// Batch fetch leverage-info для інструментів з lever=0
for k := range needsLever {
    o.get("/api/v5/account/leverage-info", map[string]string{
        "instId":  k.instId,
        "mgnMode": k.mgnMode,
    }, creds, &levResp)
    leverageByInst[k] = int(parseFloat(levResp.Data[0].Lever))
}
```

```tsx
// Frontend: "Cross" замість "0x"
const levLabel = lev > 0
  ? e.leverage
  : e.margin_mode === 'cross' ? 'Cross' : (e.margin_mode || '—')
```

**Урок:**
OKX cross-margin — leverage не збережається у positions-history. Для historical cross-margin угод leverage або невідомий, або береться поточне налаштування з leverage-info. В UI завжди показувати тип маржі ("Cross") коли конкретне плече недоступне.

**Зачеплені файли:** `internal/services/exchange/okx.go`, `frontend/src/pages/Portfolio.tsx`

---

## P-035 — OKX `closeTotalPos` — кількість в контрактах, а не в монетах; `ctVal` не враховується

**Коли:** Відображення маржі в закритих угодах — значення були неправильними на 10x–1000x.

**Проблема:**
OKX поле `closeTotalPos` в `positions-history` — це **кількість контрактів**, а не кількість базової монети. Кожен інструмент має свій `ctVal` (contract value):
- `ONDO-USDT-SWAP`: ctVal = 10 ONDO на контракт
- `ZEC-USDT-SWAP`: ctVal = 0.01 ZEC на контракт
- `XPL-USDT-SWAP`: ctVal = 10 XPL на контракт

Ми розраховували notional як `qty × entryPrice`, де `qty = closeTotalPos` (кількість контрактів):
```go
// Неправильно:
notionalUsd = math.Abs(qty) * parseFloat(item.OpenAvgPx)
// ONDO: 37 контрактів × $0.339 = $12.54 (треба $125.4)
```

Поле `notionalUsd` в positions-history або повертається як 0, або не враховується — тому fallback давав неправильний результат.

Додатково фронтенд **переобраховував** маржу з `e.quantity * e.entry_price` — та ж сама помилка.

**Рішення:**
Коли `notionalUsd = 0`, звертатись до публічного API `/api/v5/public/instruments?instType=SWAP&instId={}` → поле `ctVal`. Потім:
```go
// Правильно:
ctVal := ctValByInst[item.InstId]  // наприклад, 10 для ONDO
notionalUsd = math.Abs(qty) * ctVal * parseFloat(item.OpenAvgPx)
// ONDO: 37 × 10 × $0.339 = $125.43 ✓
```

Фронтенд тепер показує `e.max_size` напряму (notionalUsd з БД, вже правильне після sync):
```tsx
// Не перераховуємо — довіряємо значенню з БД
const margin = e.max_size
```

**Урок:**
Для OKX SWAP контрактів **ніколи** не рахувати notional як `qty × price`. `qty` — завжди в контрактах, де 1 контракт ≠ 1 монета. Або використовувати `notionalUsd` з API (якщо не 0), або фетчити `ctVal` з `/public/instruments`. Зберігати в `max_size` (position_history) завчасно відконвертоване значення в USD.

**Зачеплені файли:** `internal/services/exchange/okx.go`, `internal/services/sync_service.go`, `frontend/src/pages/Portfolio.tsx`

---

## P-036 — `npm run build` локально не оновлює frontend в Docker

**Коли:** Кожного разу після змін у frontend при роботі з Docker Compose.

**Проблема:**
Запуск `npm run build` у папці `frontend/` локально оновлює `frontend/dist/` — але Docker контейнер `frontend` зібраний з окремим образом (nginx), який не монтує локальний `dist/`. Зміни у React коді після `docker compose up` не з'являються в браузері навіть після `Ctrl+F5`.

```bash
# Це НЕ оновлює Docker контейнер:
cd frontend && npm run build
# dist/ на диску оновлено, але контейнер nginx — ні
```

**Рішення:**
Після змін у frontend (або Go-коді) необхідно перезібрати Docker образи і перезапустити контейнери:
```bash
# Зібрати нові образи:
docker compose build frontend market-data trading

# Перезапустити контейнери:
docker compose up -d frontend market-data trading
```
Після цього — hard refresh у браузері (`Ctrl+Shift+R`).

**Урок:**
При роботі з Docker Compose ніколи не плутати локальний build з build всередині контейнера. Зміни набирають чинності лише після `docker compose build <service>` + `docker compose up -d <service>`. Для швидкої розробки frontend — використовувати `npm run dev` (Vite dev server на :5173) без Docker, а збирати Docker образ тільки при релізі.

**Зачеплені файли:** `docker-compose.yml`, `frontend/Dockerfile`

---

## P-037 — Hardcoded margin_mode "cross" у sync_service.go

**Коли:** Фаза 17.2, дослідження чому колонка "Margin" у History показує обсяг позиції замість маржі.

**Проблема:**
`processPositions` (рядок 370) і `processHistory` (рядок 410) хардкодили `MarginMode: "cross"`, ігноруючи реальне значення з біржових адаптерів. OKX вже парсив `mgnMode` у `posHistItem` і заповнював `Position.MarginType`, але sync_service їх не використовував.

Додатково, колонка "Margin" на фронті показувала `e.max_size` (notional USD = обсяг позиції), а не реальну виділену маржу (`notional / leverage`). Коли плече невідоме (0), формула не може обчислити маржу — тому показується "—".

**Рішення:**
1. Додано `MarginMode` поле в `exchange.ClosedTrade` — OKX, Binance, Bybit заповнюють його.
2. `processPositions` тепер бере `p.MarginType`, `processHistory` — `t.MarginMode` (з фолбеком "cross").
3. Нова колонка `margin DECIMAL(20,8)` у `position_history` (міграція 000015) — зберігає `maxSize / leverage`.
4. Frontend: окрема колонка "Size" для обсягу, "Маржа" показує реальну маржу + тип (Cross/Isolated).

**Урок:**
Ніколи не хардкодити значення, які приходять з API. Навіть якщо "зараз все cross" — це зміниться, як тільки юзер відкриє isolated-позицію. Зберігати все, що повертає біржа, навіть якщо UI поки не використовує.

**Зачеплені файли:** `internal/services/exchange/interface.go`, `internal/services/exchange/okx.go`, `internal/services/exchange/binance.go`, `internal/services/exchange/bybit.go`, `internal/services/sync_service.go`, `internal/services/sync_repository.go`, `internal/models/portfolio.go`, `migrations/000015_history_margin.up.sql`, `frontend/src/api.ts`, `frontend/src/pages/Portfolio.tsx`

---

## P-038 — `margin_mode` не оновлюється в `ON DUPLICATE KEY UPDATE` (position_history)

**Коли:** Фаза 18, дослідження чому `margin_mode` завжди "cross" у position_history.

**Проблема:**
`insertHistory` (sync_repository.go) виконує `INSERT ... ON DUPLICATE KEY UPDATE`,
але блок `UPDATE` **не містив** `margin_mode`:

```sql
ON DUPLICATE KEY UPDATE
    leverage   = IF(VALUES(leverage) != '0x', VALUES(leverage), leverage),
    fee        = ...,
    margin     = ...,
    opened_at  = ...,
    max_size   = VALUES(max_size)
    -- margin_mode ВІДСУТНІЙ!
```

Рядки, вставлені зі старим хардкодом `"cross"` (до виправлення P-037), ніколи не оновлювались
на реальне значення, навіть коли наступний синк приносив правильний `margin_mode`.

**Рішення:**
Додати `margin_mode = VALUES(margin_mode)` до `ON DUPLICATE KEY UPDATE`.

**Урок:**
При додаванні нових колонок або виправленні значень — завжди перевіряти не лише `INSERT`,
але й `ON DUPLICATE KEY UPDATE`. Пропущена колонка в UPDATE — це "невидимий" баг:
дані вставляються правильно для нових рядків, але ніколи не виправляються для існуючих.

**Зачеплені файли:** `internal/services/sync_repository.go`

---

## P-039 — margin = 0 для угод з fallback leverage (position_history)

**Коли:** Фаза 18, дослідження чому margin = 0 для деяких OKX угод у position_history.

**Проблема:**
`processHistory` (sync_service.go:415) обчислював `margin = maxSize / leverage` **до** виклику
`insertHistory`, але `insertHistory` підтягував leverage з `open_positions` через fallback:

```go
// sync_service.go — обчислення маржі з leverage=0 → margin=0
if t.Leverage > 0 {
    margin = maxSize / float64(t.Leverage)  // OKX: lever=0 → не виконується
}

// sync_repository.go — insertHistory підставляє реальне плече з open_positions
if h.Leverage == "0x" {
    h.Leverage = lev  // "10x" з open_positions
}
// АЛЕ margin вже зафіксований як 0!
```

Тобто leverage виправлявся, а margin — ні.

**Рішення:**
Перенести перерахунок маржі в `insertHistory` — після патчу `h.Leverage`:
```go
if h.Margin == 0 {
    lev := parseLeverageStr(h.Leverage)
    if lev > 0 { h.Margin = h.MaxSize / lev }
}
```
У `processHistory` лишити обчислення лише як первинне (коли плече відоме одразу).

**Урок:**
Обчислення похідних значень (margin) повинні відбуватися **після** всіх fallback-підстановок
(leverage з open_positions). Інакше fallback виправляє вхідні дані, а похідне значення
залишається розрахованим на основі старих (неправильних) вхідних.

**Зачеплені файли:** `internal/services/sync_repository.go`, `internal/services/sync_service.go`

---

## P-040 — Реальна маржа з біржі губиться (exchange.Position не має поля Margin)

**Коли:** Фаза 18, дослідження чому margin в open_positions завжди 0.

**Проблема:**
Біржові адаптери (okx.go, binance.go, bybit.go) вже парсили поля з реальною маржею:
- OKX: `margin`, `imr` з `/api/v5/account/positions`
- Binance: `isolatedMargin` з `/fapi/v2/positionRisk`
- Bybit: `positionIM` з `/v5/position/list`

Але `exchange.Position` не мав відповідних полів, тому значення **відкидались** при створенні структури.

Для ізольованих позицій з долитою вільною маржею формула `notional/leverage` давала **неправильний**
результат — реальна маржа більша за notional/leverage (бо юзер вручну додав маржу).

При закритті позиції (deleteStalePositions бачить що біржа її більше не повертає) останнє відоме
значення margin зникало разом з рядком — position_history не мав звідки його взяти.

**Рішення:**
1. Додати `Margin float64` і `InitialMargin float64` в `exchange.Position`
2. OKX — заповнити з `p.Margin` і `p.Imr`
3. Binance — заповнити з `isolatedMargin` (fallback `notional/leverage` для cross)
4. Bybit — заповнити з `positionIM`
5. `processPositions` — зберігати `p.Margin` у `open_positions`
6. Новий метод `transferMarginToHistory` — перед `deleteStalePositions` переносить
   останнє відоме margin у position_history
7. `notional/leverage` залишається лише як фолбек для позицій без даних від біржі
8. Міграція 000016 (маркер), міграція 000017 (бекфіл margin для існуючих рядків history)

**Урок:**
Якщо біржа повертає поле — зберігати його, навіть якщо поки є обчислювальний fallback.
Для ізольованих позицій з manual margin add формула `notional/leverage` = неправильна.
Перед видаленням рядка з однієї таблиці — перенести потрібні дані в пов'язану таблицю.

**Зачеплені файли:** `internal/services/exchange/interface.go`, `internal/services/exchange/okx.go`,
`internal/services/exchange/binance.go`, `internal/services/exchange/bybit.go`,
`internal/services/sync_service.go`, `internal/services/sync_repository.go`,
`migrations/000016_open_positions_margin.up.sql`, `migrations/000017_backfill_history_margin.up.sql`,
`frontend/src/pages/Portfolio.tsx`

---

## P-041 — Безумовний DELETE stale після мовчазної помилки API біржі

**Коли:** Фаза 18, дослідження зникнення відкритих позицій на вкладці Futures.

**Проблема:**
`SyncPositions` (sync_service.go) ігнорував помилку з `ex.GetOpenPositions()`:
```go
if positions, err := ex.GetOpenPositions(c.creds); err == nil {
    s.processPositions(...)
}
// err != nil — мовчки ігнорується, жодного логу
```
А `deleteStalePositions(userID, syncStart)` виконувався **після** WaitGroup для **всіх** бірж
одночасно:
```go
s.repo.deleteStalePositions(userID, syncStart) // DELETE WHERE user_id=? AND updated_at < ?
```
Один таймаут OKX → processPositions не виконується → updated_at не оновлюється →
DELETE видаляє всі позиції цієї біржі. Те саме для SyncUser і SyncBalances.
З інтервалом sync-live 45 с це стало помітно — позиції зникали і з'являлись.

**Рішення:**
1. `deleteStalePositions` і `deleteStaleBalances` — додано параметр `exchange`:
   ```sql
   DELETE FROM open_positions WHERE user_id = ? AND exchange = ? AND updated_at < ?
   ```
2. Stale cleanup перенесено **всередину горутини** кожної біржі, одразу після
   успішного processPositions / processBalances.
3. При помилці — логування через `s.logger.Warn` і інкремент Prometheus
   `sync_exchange_errors_total{exchange, operation}`.
4. Скопійовано патерн з `syncFuturesExchange` де cleanup вже був per-exchange.

**Урок:**
Ніколи не виконувати безумовний DELETE на основі timestamp після операції, яка може мовчки
провалитись. Патерн "fetch → upsert → delete stale" вимагає що DELETE відбувається
**тільки при успішному fetch**, і **тільки для конкретної біржі**, а не для всіх одразу.
Додатково — завжди логувати помилки з API бірж. "Мовчазна помилка + безумовний cleanup"
= гарантоване видалення даних при першому ж таймауті.

**Зачеплені файли:** `internal/services/sync_service.go`, `internal/services/sync_repository.go`,
`internal/metrics/metrics.go`

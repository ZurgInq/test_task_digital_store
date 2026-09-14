# Тестовое задание "ядро магазина цифровых товаров"

Приложение эмулирует backend магазина с оплатой товаров. Демонстрирует возможность восстановление состояние заказов при различных сбоях.

Реализовано:
* Создание заказа по API запросу.
* Взаимодействие с сервисом платежей (заглушка).
* Взаимодействие с провайдером товаров (заглушка).
* Восстановление состояние заказов после различных сбоев.

Не реализовано:
* Валидация данных.
* Реальные операции с балансом.
* Тесты.

Код "живой" - не рефакторился, не однородный, не является идиоматически верным для go.

## Стэк

Проект носит демонстрационный характер, не production решение. Запускаем с минимальным количеством внешних зависимостей.

* Язык - go lang.
* БД: sqlite.
* Очереди: эмуляция in memory в приложении.

Основные фрэймворки и библиотеки:
* WEB: chi
* ORM: gorm

Ограничения накладываемые sqlite по возможности обходятся архитектурой приложения.

## Запуск приложения

**Ручной запуск (требует go 1.27.1)**

Занимаемые порты по умолчанию 3000-3004. Пути указаны относительно корня проекта.

```bash
# Запуск основного приложения

cd app && make run

# Запуск заглушек

# Сервис платежей
cd mocks/ && go run ./payments/main.go

# Сервис выдачи кодов
cd mocks/ && go run ./codes/main.go

# Сервис выдачи кодов fallback
cd mocks/ && APP_PORT=3003 go run ./codes/main.go

# Сервис выдачи кодов для GIFT товаров
cd mocks/ && APP_PORT=3004 APP_TEST_DUPLICATE_CODE=XXX-YYY-ZZZ go run codes/main.go

# Генерируем пул кодов

curl -X POST localhost:3002/generate/codes
```

Настройка количества сбоев при выдаче кодов (подробнее смотри ниже [Заглушка сервиса выдачи кодов](#заглушка-сервиса-выдачи-кодов)):
```bash
# 30% ответов - ошибка 500
# 20% ответов с задержкой в 10 секунд
APP_TEST_ERRORS_P=30 APP_TEST_TIMEOUT_P=20 go run codes/main.go
```

Основное приложение предоставляет простую тестовую страницу по адресу http://localhost:3000/. Доступное API для ручного тестирования описано далее. Примеры запросов лежат в директории `test_api`.

**Cброс состояния**

* Останавливаем все сервисы.
* Удаляем файлы БД:
    - mocks/payments.db
    - mocks/codes.db
    - app/app.db

## Структура проекта

* app/ - основное приложение с http api для создания заказов.
* mocks/ - заглушки:
    * codes/ - заглушка поставщика выдачи товаров.
    * payments/ - заглушка сервиса платежей.
* test_api/ - curl скрипты для тестирования API.

## Основное приложение

Директория app/

* actions - основные use cases.
* cmd/
    * api/ - точка входна основоного приложения.
    * migrate/ - создание схемы для БД.
* db/ - минимальная обёртка над gorm для транзакций.
* migrations/ - sql файлы миграций.
* models/ - модели и репозитории.
* queue/ - эмуляция очереди. Сгенерировано через LLM.
* seed_data/ - файл для заполнения БД.
* static/ - статиа, страница для ручного тестирования API.
* vendor/ - вендоринг
* web/ - API роутинг и попутная простая логика.

Старт приложения через `make migrate` - запускает миграцию и стартует приложение.

Ручной старт миграции: `go run ./cmd/migrate/`.

Ручной старт приложения: `go run ./cmd/api/`.

При старте приложения наполняется таблица `products`. По умолчанию веб сервер стартует на порту `3000`.

Переменные окружения для настройки и значения по умолчанию:
* APP_HOST
* APP_PORT=3000
* SEED_DATA_DIR=seed_data
* PAYMENT_ADDR=http://localhost:3001 адрес заглушки сервиса платежей
* ISSUE_API_ADDR_MAIN=http://localhost:3002 адрес заглушки сервиса выдачи кодов
* ISSUE_API_ADDR_FALLBACK=http://localhost:3003 фолбэк адрес
* GIFT_CARD_API_ADDR_MAIN=http://localhost:3004 адрес заглушки для gift кодов


По адресу http://localhost:3000/ отдаётся тестовая страница.

### Схема данных

Миграции - `app/migrations/*`.

```go
// Заказ
type Order struct {
	gorm.Model

	GroupID   GroupID // ИД для группировки заказов из нескольких товаров
	ExtID     string  // ИД для внешних сервисов
	UserID    uint
	Status    Status
	ProductID uint
	Amount    currency.Amount // Зафиксированная сумма для отмены заказа
	Code      string          // Выданный код
}
```

```go
// Результат запроса кода из сервиса поставщика
type Issue struct {
	gorm.Model

	RequestID     string        // ИД для идемпотентности
	SKU           string        //
	OrderExtID    string        // ИД заказа
	Code          string        // Выданный код
	Status        Status        // Результат api запроса ok/error
	ErrReason     string        // Код ошибки api
	ApiAddr       string        // Адрес сервиса api. Для повторных запросов в случае таймаутов.
	RequestStatus RequestStatus // Результат http запроса: ok/timeout/unavailable
	ResponseCode  int           // Код http
}
```

```go
// Платёж
type Payment struct {
	gorm.Model

	EventID      *string // ИД для идемпотентности
	UserID       uint
	OrderGroupID orderPkg.GroupID
	Status       Status // Статус платежа paid/failed
	Amount       currencyPkg.Amount
	Currency     currencyPkg.Currency
}
```

### Архитектура приложения

Большинство операция носит ассинхронный характер. Для взаимодействия с внешними сервисами используется http api. Основные модели - `Order` и `Payment`. Бизнес сценарии вынесены в директорию `actions/*`. Для `Order` генерируется случайный строковый `ExtID` для взаимодействия с внешними сервисами. Вспомогательная модель `Issue` логирует запросы на получение кодов и предотвращает дублирование запросов.

Для исключения гонок данных внутри приложения используется in-memory эмуляция очереди с обработкой в один поток. В production коде необходимо использовать очереди на основе решений вроде kafka/rabbitmq. Для распределённых локов - redis или конструкцию select for update.

Повторные http запросы реализованы только в сервис выдачи кодов. Настройки хардкодом в `app/models/issue/api_client.go`:
```go
// backoff settings
var (
	BackoffInitialInterval = 1 * time.Second
	BackoffMaxTries        = uint(5)
)

// http client settings
var (
	ClientTLSHandshakeTimeout = 2 * time.Second
	ClientDialTimeout         = 2 * time.Second
	ClientDialKeepAlive       = 30 * time.Second
	ClientTimeout             = 8 * time.Second
)
```

### Потоки данных

**Создание заказа через http api:**
- Создаётся запись в таблице `orders`. Заказ получает статус `created`.
- Создаётся платёж в таблице `payments` со статусом `init`.
- Публикуется событие с созданным платежом в очередь `payments:init`.

**Старт оплаты, читаем события из `payments:init`:**
- Отправляем запрос на создание платежа.
- Переводим платёж в статус `await`.

**Внешний сервис платежей, оплата**
- Отправляется запрос с результатом оплаты в основное приложение на `/webhook/payments`.

**Оплата, обработка веб хука:**
- Если существует старый платёж с переданным `event_id`, то пропускаем его.
- Ищем платёж по заказу в статусе `await`, меняем статус на `paid` или `failed`.
- Сохраняем `event_id` в платёж.
- Создаём денежную транзакцию `deposit`.
- Меняем статус заказа.
- Для заказов в статусе `paid` публикуем событие в очередь `orders:paid`.

**Получение кода после оплаты, читам события из `get-issue:`**
- Заказ переводится в статус `delivering`.
- Выполняем запрос в сервис поставщика товаров.
    - Результат запроса сохраняется в таблицу `issues`. Таблица используется для контроля повторных запросов.
- В случае успеха статус заказа переводится в `delivered` и в заказ сохраняется полученный код.

### Восстановление заказов

Восстановление заказов выполняется вручную по запросу API. Автоматическое переодические восстановление (например, из статуса `out_of_stock` или `delivery_failed`) не реализовано для упрощения.

**Ограничения по статусу оплаты**

Не реализован опрос сервиса платежа для получения статуса оплаты. Статус платежа должен быть получен только через веб хук. Повторная оправка веб хука - `test_api/payment_webhook.sh`.

### Описание API

Основные роуты (`app/web/api.go`):

```go
    // orders
	r.Route("/api/orders", func(r chi.Router) {
		r.Post("/", api.createOrder)
		r.Get("/", api.getOrders)
		r.Get("/{orderId}", api.getOrder)
		r.Get("/{orderId}/issues", api.getOrdersIssues)
	})

	// payments
	r.Route("/webhook/payments", func(r chi.Router) {
		r.Post("/", api.createPayment)
	})

	// recovery
	r.Route("/api/recovery", func(r chi.Router) {
		r.Post("/orders", api.recoveryOrders)
	})
```

Примеры запросов:
* Создание заказа  - `test_api/create_order.sh`.
* Восстановление заказов - `test_api/recovery_orders.sh`.
* Повторная отправка веб хука по оплате (через сервис платежей) - `test_api/payment_webhook.sh`. 

## Заглушка сервиса платежей

Директория mocks/payments/

Ручной запуск из директории `mocks/` - `go run payments/main.go`. По умолчанию запускается на порту `3001`.

Переменные окружения для настройки и значения по умолчанию:
* APP_HOST
* APP_PORT=3001
* APP_CALLBACK_ADDR=http://localhost:3000

При старте создаётся БД `payments.db`, схема данных создаётся автоматически, внешних sql файлов для заглушки нет.

Сервис отвечает за эмуляцию оплаты за товар.
* Создание ожидания платежа - `POST /api/payments/`.
* Статус оплаты меняется только вручную через запрос `POST /{orderID}/status/{status}`.
    - Автоматически отправляется веб хук в основное приложение.
* Вручную отправляем повторные веб хуки через `/{orderID}/webhook`.

### Описание API

Основные роуты (mocks/payments/main.go):
```go
r.Route("/api/payments", func(r chi.Router) {
    // Инициализация оплаты
    r.Post("/", func(w http.ResponseWriter, r *http.Request){})
    // Смена статуса оплаты
    r.Post("/{orderID}/status/{status}", func(w http.ResponseWriter, r *http.Request){})
    // Повторная отправка вебхука в основное приложение
    r.Post("/{orderID}/webhook", func(w http.ResponseWriter, r *http.Request) {})
})
```

Примеры запросов:
* Смена статуса заказа - `test_api/finish_payment.sh ord_FsCYXdnm7icr paid`.
* Повторная отправка веб хука - `test_api/payment_webhook.sh ord_FsCYXdnm7icr`.

## Заглушка сервиса выдачи кодов

Директория mocks/codes/

Ручной запуск из директории `mocks/` - `go run codes/main.go`. По умолчанию запускается на порту `3002`.

Переменные окружения для настройки и значения по умолчанию:
* APP_HOST
* APP_PORT="3002"
* APP_TEST_TIMEOUT_SEC=10 - Эмуляция долгой обработки запроса.
* APP_TEST_TIMEOUT_P=0 - Процент ответов с таймаутом APP_TEST_TIMEOUT_SEC.
* APP_TEST_ERRORS_P=0 - Процент ответов с ошибкой 500.
* APP_TEST_DUPLICATE_CODE - Если указано, то на любой запрос выдаёт данный код.

При старте создаётся БД `codes.db`, схема данных создаётся автоматически, внешних sql файлов для заглушки нет. **БД стартует с пустым списком кодов для выдачи**. Необходимо наполнить БД кодами через АПИ запрос (`test_api/generate_codes.sh`).

Сервис отвечает за выдачу кодов основному приложению. Через переменные окружения можно настроить процент сбойных запросов.

### Описание API

```go
r.Route("/issue", func(r chi.Router) {
    // Выдача кода
    r.Post("/", func(w http.ResponseWriter, r *http.Request){})
}

r.Route("/issues", func(r chi.Router) {
    // Список запросов на получение кодов
    r.Get("/", func(w http.ResponseWriter, r *http.Request) {})
})

r.Route("/codes", func(r chi.Router) {
    // Список всех кодов
    r.Get("/", func(w http.ResponseWriter, r *http.Request) {})
})

r.Route("/generate/codes", func(r chi.Router) {
    // Генерация нового пула кодов
	r.Post("/", func(w http.ResponseWriter, r *http.Request) {})
}()
```

Примеры запросов:
* Генерация пула кодов - `test_api/generate_codes.sh`.

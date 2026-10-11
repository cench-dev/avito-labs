# TripGo — репозиторий для лабораторных работ

## Запуск

Из корня репозитория выполнить:
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect

Применить миграции:
make migrate

Запустить сервис:
make run

## Переменные окружения
HTTP_ADDR - адрес HTTP-сервера.

DATABASE_URL - строка подключения к PostgreSQL.

LOG_LEVEL - уровень логирования.

SHUTDOWN_TIMEOUT - таймаут graceful shutdown.

DATABASE_MAX_CONNS - максимальное количество соединений с БД.

DATABASE_MIN_CONNS - минимальное количество соединений с БД.

DATABASE_CONNECT_TIMEOUT - таймаут подключения к БД.

DATABASE_QUERY_TIMEOUT - таймаут выполнения запросов к БД.

DATABASE_MAX_CONN_LIFETIME - максимальное время жизни соединения.

## Миграции
Применить все ожидающие миграции:

make migrate

Откатить последнюю миграцию:

goose -dir migrations postgres "$DATABASE_URL" down

## Решения
### Уровень изоляции транзакций - Read Commited 
Запрет двух активных поездок у одного водителя обеспечивается уникальным частичным индексом по driver_id для строк со статусом active. Поэтому ограничение сохраняется и при одновременных запросах. Ошибка PostgreSQL с кодом 23505 преобразуется в доменную ошибку driver_busy.


### Менеджер транзакций
Интерфейс:
type TxManager interface {
    Do(ctx context.Context, fn func(ctx context.Context) error) error
}


Метод Do открывает транзакцию и помещает её в контекст, передаваемый функции fn. Если функция завершается без ошибки, менеджер выполняет COMMIT. При ошибке или панике выполняется ROLLBACK, а ошибка или паника передаётся дальше.


### Проверка работоспособности
/health возвращает 200 OK, если процесс работает, и не обращается к базе данных.

/ready проверяет доступность PostgreSQL и возвращает 503 Service Unavailable, если база данных недоступна.

При получении SIGINT или SIGTERM сервер выполняет graceful shutdown с ограничением времени из SHUTDOWN_TIMEOUT.


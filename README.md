## Описание проекта
В данном проекте реализован API для мерч-стора.
При успешной аутентификации или регистрации пользователям выдается JWT-токен доступа. Реализовано также автоматическое создание пользователя при первой авторизации.

В рамках проекта реализовано покрытие бизнес-сценариев юнит-тестами (общее покрытие проекта превышает 40%) и интеграционными тестами. Основные сценарии интеграционных тестов:
* Покупка мерча
* Передача монеток другим сотрудникам
* Получение информации по истории транзакций

Дополнительно реализована конфигурация линтера — файл .golangci.yaml в корне проекта.

## Запуск проекта
Для запуска проекта выполните следующие шаги.
Перейдите в директорию проекта.
```bash
cd merch_store
```
Запустите контейнеры через docker-compose.
```bash
docker-compose up -d
```
После этого сервис будет доступен на порту :8080.

Тестирование
Юнит-тесты реалезованы и представлены в файле handlers_test.go.
```
/merch_store/internal/service/handlers_test.go
```
Для реализации юнит-тестов использовался gomock.
```
/merch_store/internal/storage/mocks/mock_postgresql.go
```
Интеграционные тесты реализованы и представлены в файле integration_test.go
```
/merch_store/tests/integration/integration_test.go
```
Интеграционные тесты запускаются из корня проекта с помощью следующей команды:

```bash
make test.integration
```
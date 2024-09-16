# Сервис проведения тендеров

## Запуск

Приложение можно запустить при помощи docker-compose.

Важно: необходимо иметь настроенные переменные окружения в файле .env и перед запуском тестов инициализировать данные таблиц как описано ниже. Реализация хэндлеров для обновления этих данных в рамках проекта не предполагалась.  

## To run the docker image

```bash
docker-compose up --build
```

Команда соберет проект и автоматически инициализирует нужные таблицы из /internal/db/init.sql.

## Реализованный функционал 
| Название группы    | Ручки                                  
| ------------------ | -------------------------------------- 
| 01/ping            | - /ping
| 02/tenders/new     | - /tenders/new
| 03/tenders/list    | - /tenders<br>- /tenders/my
| 04/tenders/status  | - /tenders/status
| 05/tenders/version | - /tenders/edit
| 06/bids/new        | - /bids/new
| 07/bids/list      | /bids/my

## Запуск тестов

```bash
go test ./...
```


### Стек
- Golang
- Postgres

### Настройка приложения производится через переменные окружения

Переменные настраивались через файл .env

- `SERVER_ADDRESS` — 8080
- `POSTGRES_CONN` — URL-строка для подключения к PostgreSQL в формате postgres://{username}:{password}@{host}:{5432}/{dbname}.
- `POSTGRES_JDBC_URL` — JDBC-строка для подключения к PostgreSQL в формате jdbc:postgresql://{host}:{port}/{dbname}.
- `POSTGRES_USERNAME` — имя пользователя для подключения к PostgreSQL.
- `POSTGRES_PASSWORD` — пароль для подключения к PostgreSQL.
- `POSTGRES_HOST` — IP docker-контейнера
- `POSTGRES_PORT` — 5432
- `POSTGRES_DATABASE` — имя базы данных PostgreSQL, которую будет использовать приложение.

В рамках тестирования также заполнялись данные таблиц, пример скрипта для pgAdmin:

```sql
DO $$
DECLARE
    employee_id UUID;
BEGIN
    INSERT INTO employee (username, first_name, last_name)
    VALUES ('test_user', 'Test', 'User')
    RETURNING id INTO employee_id;

    INSERT INTO organization (id, name, description, type)
    VALUES ('550e8400-e29b-41d4-a716-446655440000', 'string', 'string', 'LLC');

    INSERT INTO organization_responsible (organization_id, user_id)
    VALUES ('550e8400-e29b-41d4-a716-446655440000', employee_id);
END $$;
```

## Основные требования
### Сущности
#### Пользователь и организация

Сущности пользователя и организации уже созданы и представлены в базе данных следующим образом:

Пользователь (User):

```sql
CREATE TABLE employee (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(50) UNIQUE NOT NULL,
    first_name VARCHAR(50),
    last_name VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

Организация (Organization):
```sql
CREATE TYPE organization_type AS ENUM (
    'IE',
    'LLC',
    'JSC'
);

CREATE TABLE organization (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    type organization_type,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE organization_responsible (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID REFERENCES organization(id) ON DELETE CASCADE,
    user_id UUID REFERENCES employee(id) ON DELETE CASCADE
);
```

### API
Все эндпоинты начинаются с префикса /api.

Обратите внимание, что успешное выполнение запроса GET /api/ping обязательно для начала тестирования приложения.

Все запросы и ответы должны соответствовать структуре и требованиям спецификации Open API, включая статус-коды, ограничения по длине и допустимые символы в строках.

Если запрос не соответствует требованиям, возвращайте статус-код 400. Если же имеется более специфичный код ответа, используйте его.

Если в запросе есть хотя бы один некорректный параметр, весь запрос должен быть отклонён.

### Бизнес-логика
#### Тендер

Тендеры могут создавать только пользователи от имени своей организации.

Доступные действия с тендером:

- **Создание**:

  - Тендер будет создан.

  - Доступен только ответственным за организацию.

  - Статус: `CREATED`.

- **Публикация**:

  - Тендер становится доступен всем пользователям.

  - Статус: `PUBLISHED`.

- **Закрытие**:

  - Тендер больше не доступен пользователям, кроме ответственных за организацию.

  - Статус: `CLOSED`.

- **Редактирование**:

  - Изменяются характеристики тендера.

  - Увеличивается версия.

#### Предложение

Предложения могут создавать пользователи от имени своей организации.

Предложение связано только с одним тендером. Один пользователь может быть ответственным в одной организации.

Доступные действия с предложениями:

- **Создание**:

  - Предложение будет создано.

  - Доступно только автору и ответственным за организацию.

  - Статус: `CREATED`.

- **Публикация**:

  - Предложение становится доступно ответственным за организацию и автору.

  - Статус: `PUBLISHED`.

- **Отмена**:

  - Виден только автору и ответственным за организацию.

  - Статус: `CANCELED`.

- **Редактирование**:

  - Изменяются характеристики предложения.

  - Увеличивается версия.

- **Согласование/отклонение**:

  - Доступно только ответственным за организацию, связанной с тендером.

  - Решение может быть принято любым ответственным.

  - При согласовании одного предложения, тендер автоматически закрывается.

## Дополнительные требования

1. Расширенный процесс согласования:

   - Если есть хотя бы одно решение reject, предложение отклоняется.
   
   - Для согласования предложения нужно получить решения больше или равно кворуму.
   
   - Кворум = min(3, количество ответственных за организацию).

3. Просмотр отзывов на прошлые предложения:

   - Ответственный за организацию может просмотреть отзывы на предложения автора, который создал предложение для его тендера.

5. Оставление отзывов на предложение:

   - Ответственный за организацию может оставить отзыв на предложение.

7. Добавить возможность отката по версии (Тендер и Предложение):

   - После отката, считается новой правкой с увеличением версии.

9. Описание конфигурации линтера.

## Тестирование

### 1. Проверка доступности сервера
- **Эндпоинт:** GET /ping
- **Цель:** Убедиться, что сервер готов обрабатывать запросы.
- **Ожидаемый результат:** Статус код 200 и текст "ok".

```yaml
GET /api/ping

Response:

  200 OK

  Body: ok
```

### 2. Тестирование функциональности тендеров
#### Получение списка тендеров
- **Эндпоинт:** GET /tenders
- **Описание:** Возвращает список тендеров с возможностью фильтрации по типу услуг.
- **Ожидаемый результат:** Статус код 200 и корректный список тендеров.

```yaml
GET /api/tenders

Response:

  200 OK

  Body: [ {...}, {...}, ... ]
```

#### Создание нового тендера
- **Эндпоинт:** POST /tenders/new
- **Описание:** Создает новый тендер с заданными параметрами.
- **Ожидаемый результат:** Статус код 200 и данные созданного тендера.

```yaml
POST /api/tenders/new

Request Body:

  {

    "name": "Тендер 1",

    "description": "Описание тендера",

    "serviceType": "Construction",

    "status": "Open",

    "organizationId": 1,

    "creatorUsername": "user1"

  }

Response:

  200 OK

  Body: 
  
  { 
    "id": 1, 
    "name": "Тендер 1", 
    "description": "Описание тендера",
    ...
  }
```

#### Получение тендеров пользователя
- **Эндпоинт:** GET /tenders/my
- **Описание:** Возвращает список тендеров текущего пользователя.
- **Ожидаемый результат:** Статус код 200 и список тендеров пользователя.

```yaml
GET /api/tenders/my?username=user1

Response:

  200 OK

  Body: [ {...}, {...}, ... ]  
```

#### Редактирование тендера
- **Эндпоинт:** PATCH /tenders/{tenderId}/edit
- **Описание:** Изменение параметров существующего тендера.
- **Ожидаемый результат:** Статус код 200 и обновленные данные тендера.

```yaml
PATCH /api/tenders/1/edit

Request Body:

  {

    "name": "Обновленный Тендер 1",

    "description": "Обновленное описание"

  }

Response:

  200 OK

  Body: 
  { 
    "id": 1, 
    "name": "Обновленный Тендер 1", 
    "description": "Обновленное описание",
    ...
  }  
```

#### Откат версии тендера
- **Эндпоинт:** PUT /tenders/{tenderId}/rollback/{version}
- **Описание:** Откатить параметры тендера к указанной версии.
- **Ожидаемый результат:** Статус код 200 и данные тендера на указанной версии.

```yaml
PUT /api/tenders/1/rollback/2

Response:

  200 OK

  Body: 
  { 
    "id": 1, 
    "name": "Тендер 1 версия 2", 
    ... 
  }
```

### 3. Тестирование функциональности предложений
#### Создание нового предложения
- **Эндпоинт:** POST /bids/new
- **Описание:** Создает новое предложение для существующего тендера.
- **Ожидаемый результат:** Статус код 200 и данные созданного предложения.

```yaml
POST /api/bids/new

Request Body:

  {

    "name": "Предложение 1",

    "description": "Описание предложения",

    "status": "Submitted",

    "tenderId": 1,

    "organizationId": 1,

    "creatorUsername": "user1"

  }

Response:

  200 OK

  Body: 
  { 
    "id": 1, 
    "name": "Предложение 1", 
    "description": "Описание предложения",
    ...
  }
```

#### Получение списка предложений пользователя
- **Эндпоинт:** GET /bids/my
- **Описание:** Возвращает список предложений текущего пользователя.
- **Ожидаемый результат:** Статус код 200 и список предложений пользователя.

```yaml
GET /api/bids/my?username=user1

Response:

  200 OK

  Body: [ {...}, {...}, ... ]
  ```
  
#### Получение списка предложений для тендера
- **Эндпоинт:** GET /bids/{tenderId}/list
- **Описание:** Возвращает предложения, связанные с указанным тендером.
- **Ожидаемый результат:** Статус код 200 и список предложений для тендера.

```yaml
GET /api/bids/1/list

Response:

  200 OK

  Body: [ {...}, {...}, ... ]
  ```
  
#### Редактирование предложения
- **Эндпоинт:** PATCH /bids/{bidId}/edit
- **Описание:** Редактирование существующего предложения.
- **Ожидаемый результат:** Статус код 200 и обновленные данные предложения.

```yaml
PATCH /api/bids/1/edit

Request Body:

  {

    "name": "Обновленное Предложение 1",

    "description": "Обновленное описание"

  }

Response:

  200 OK

  Body: 
  { 
    "id": 1, 
    "name": "Обновленное Предложение 1", 
    "description": "Обновленное описание",
    ...,
  }
```

#### Откат версии предложения
- **Эндпоинт:** PUT /bids/{bidId}/rollback/{version}
- **Описание:** Откатить параметры предложения к указанной версии.
- **Ожидаемый результат:** Статус код 200 и данные предложения на указанной версии.

```yaml
PUT /api/bids/1/rollback/2

Response:

  200 OK

  Body: 
  { 
    "id": 1, 
    "name": "Предложение 1 версия 2", 
    ...
  }
```

### 4. Тестирование функциональности отзывов
#### Просмотр отзывов на прошлые предложения
- **Эндпоинт:** GET /bids/{tenderId}/reviews
- **Описание:** Ответственный за организацию может посмотреть прошлые отзывы на предложения автора, который создал предложение для его тендера.
- **Ожидаемый результат:** Статус код 200 и список отзывов на предложения указанного автора.

```yaml
GET /api/bids/1/reviews?authorUsername=user2&organizationId=1

Response:

  200 OK

  Body: [ {...}, {...}, ... ]
```

### Оценивание


| Название группы    | Ручки                                  | Баллы | От каких групп зависит |
| ------------------ | -------------------------------------- | ----- | ---------------------- |
| 01/ping            | - /ping                                | 1     |                        |
| 02/tenders/new     | - /tenders/new                         | 2     |                        |
| 03/tenders/list    | - /tenders<br>- /tenders/my            | 5     | - 02/tenders/new       |
| 04/tenders/status  | - /tenders/status                      | 3     | - 02/tenders/new       |
| 05/tenders/version | - /tenders/edit<br>- /tenders/rollback | 6     | - 02/tenders/new       |
| 06/bids/new        | - /bids/new                            | 2     |                        |
| 07/bids/decision   | - /bids/submit_decision                | 3/6   | - 06/bids/new          |
| 08/bids/list       | - /bids/list<br>- /bids/my             | 5     | - 06/bids/new          |
| 09/bids/status     | - /bids/status                         | 3     | - 06/bids/new          |
| 10/bids/version    | - /bids/edit<br>- /bids/rollback       | 6     | - 06/bids/new          |
| 11/bids/feedback   | - /bids/reviews<br>- /bids/feedback    | 7     | - 06/bids/new          |




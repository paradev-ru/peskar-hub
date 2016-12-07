# Peskar Hub

## Ход работы

* Запускаются peskar-hub (далее "хаб") и peskar-worker (далее "воркер");
* Воркер переодически опрашивает хаб на наличие новых заданий (`GET /ping/` -> `404 Not Found`);
* В хаб добавляется задание, ему присваивается статус `pending`;
* Воркер получает задание (`GET /ping/` -> `200 OK`);
* Хаб меняет статут задания на `requested`;
* Воркер подтверждает получение задания установкой статуса `working` (`PUT /job/{id}/` -> `200 OK`);
* Воркер начинает выполнение работ;
* Воркер переодически проверяет статус задания и отправляет логи (`PUT /job/{id}/` -> `200 OK`);
* Воркер завершает выполнение работ;
* Воркер подтверждает завершение работ установкой статуса `finished` (`PUT /job/{id}/` -> `200 OK`).

## Выполнение запросов

> Все данные должны передаваться в JSON.

### Ошибки

API может возвращать различные ошибки в следующем формате:

```json
{
    "code": 404,
    "message": "Job not found"
}
```

Значение `code` соответствует коду состояния HTTP.

## Запросы

### Версия системы

`GET /version/`

Пример ответа:

```json
"0.1.1"
```

### Проверка работы системы

`GET /health/`

Пример ответа:

```json
{
    "uptime": "4.165746051s"
}
```

### Получение нового задания

`GET /ping/`

После получения задания воркером, статус задания меняется с `pending` на `requested` и далее считается взятым в работу. Если, по истечении 5 минут, статус задания не был изменен с `requested` на любой другой (working, canceled, failed), статус меняется обратно на `pending`.

### Список заданий

`GET /job/`

Пример ответа:

```json
{
    "1CDCDE08-C716-BADC-7A3D-E492B97A80D2": {
        "id": "1CDCDE08-C716-BADC-7A3D-E492B97A80D2",
        "state": "pending",
        "download_url": "http://stormy.homeftp.net/HD/720p/Fargo_BDRip_720p.mkv",
        "name": "Fargo (720p)",
        "description": "20th Anniversary",
        "info_url": "http://weburg.net/movies/info/1795",
        "added_at": "2016-11-08T19:36:41.464841575Z",
        "started_at": "0001-01-01T00:00:00Z",
        "finished_at": "0001-01-01T00:00:00Z"
    }
}
```

### Создание задания

`POST /job/`

Параметр     | Описание
-------------|---------------------------------
name         | Название
description  | Описание
info_url     | Ссылка на страницу с информацией
download_url | Ссылка на файл загрузки

### Информация по заданию

`GET /job/{id}/`

Пример ответа:

```json
{
    "id": "1CDCDE08-C716-BADC-7A3D-E492B97A80D2",
    "state": "pending",
    "download_url": "http://stormy.homeftp.net/HD/720p/Fargo_BDRip_720p.mkv",
    "name": "Fargo (720p)",
    "description": "20th Anniversary",
    "info_url": "http://weburg.net/movies/info/1795",
    "added_at": "2016-11-08T19:36:41.464841575Z",
    "started_at": "0001-01-01T00:00:00Z",
    "finished_at": "0001-01-01T00:00:00Z"
}

```

Метод вернет `404: Job not found`, если задание по указанному `id` не найдено.

### Обновление задания

`PUT /job/{id}/`

Параметр    | Описание
------------|--------------------------------------------------------
name        | Название
description | Описание
info_url    | Ссылка на страницу с информацией
state       | Состояние задания (working, finished, canceled, failed)
log         | Логи событий

Метод вернет `404: Job not found`, если задание по указанному `id` не найдено.

### Удаление задания

`DELETE /job/{id}/`

Метод вернет `404: Job not found`, если задание по указанному `id` не найдено.

### Добавление лога в задание

`POST /job/{id}/log/`

Параметр    | Описание
------------|----------------
message     | Текст сообщения

### Получение лога задания

`GET /job/{id}/log/`

### Удаление лога задания

`DELETE /job/{id}/log/`

### Получение истории задания

`GET /job/{id}/state_history/`

### Удаление истории задания

`DELETE /job/{id}/state_history/`

### Список воркеров

`GET /worker/`

Воркер регистрируется в системе со статусом `active` при вызове метода `GET /ping/`. Если, по истечении 5 минут, воркер не совершил ни одного вызова метода `GET /ping/`, его статус меняется на `inactive`.

Пример ответа:

```json
{
    "127.0.0.1": {
        "ip": "127.0.0.1",
        "state": "active",
        "user_agent": "curl/7.49.0"
    }
}
```

### HTTP статус ссылки

`GET /http_status/`

Параметр | Описание
---------|---------------
url      | Ссылка на файл

Пример ответа:

```json
{
    "status_code": 200,
    "status": "200 Ok",
    "content_length": 9456
}
```

### Рабочее время

`GET /work_time/`

Пример ответа:

```json
{
    "dnd_enable": true,
    "dnd_ends_at": 12,
    "dnd_starts_at": 1,
    "is_work_time": true,
    "local_time": "2016-11-13T13:14:15.094261238+05:00",
    "local_time_utc": "2016-11-13T08:14:15.094261283Z"
}
```

## Статусы задач

Название  | Описание
----------|---------------------------------
pending   | В очереди на обработку
requested | Запрошено воркером
working   | В работе
canceled  | Выполнение отменено
finished  | Успешно завершено
failed    | Завершено с ошибкой
deleted   | Удалено

# Peskar Hub

## Ход работы

* Запускаются peskar-hub (далее "хаб") и peskar-worker (далее "воркер");
* Воркер переодически опрашивает хаб на наличие новых заданий (`GET /ping/`);
* В хаб добавляется задание, ему присваивается статус `pending`;
* Воркер получает задание, статус задания автоматически меняется на `requested`;
* Воркер подтверждает получение задания установкой статуса `working` (`PUT /job/{id}/`);
* Воркер начинает выполнение работ;
* Воркер переодически проверяет статус задания и отправляет логи (`PUT /job/{id}/`);
* Воркер завершает выполнение работ;
* Воркер подтверждает завершение работ установкой статуса `finished` (`PUT /job/{id}/`).

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
    "B5E119D6-01E7-E22C-6331-0B5CDA286B8A": {
        "id": "B5E119D6-01E7-E22C-6331-0B5CDA286B8A",
        "state": "pending",
        "download_url": "http://ya.ru",
        "added_at": "2016-11-07 19:49:34.446974953 +0000 UTC"
    }
}
```

### Создание задания

`POST /job/`

Параметр     | Описание
-------------|---------------------------------
name         | Название
info_url     | Ссылка на страницу с информацией
download_url | Ссылка на файл загрузки

### Информация по заданию

`GET /job/{id}/`

Пример ответа:

```json
{
    "id": "B5E119D6-01E7-E22C-6331-0B5CDA286B8A",
    "state": "pending",
    "download_url": "http://ya.ru",
    "added_at": "2016-11-07 19:49:34.446974953 +0000 UTC"
}

```

Метод вернет `404: Job not found`, если задание по указанному `id` не найдено.

### Обновление задания

`PUT /job/{id}/`

Параметр | Описание
---------|------------------------------------------------------
state    | Состояние задания (working, finished, canceled, failed)
log      | Логи событий

Метод вернет `404: Job not found`, если задание по указанному `id` не найдено.

### Удаление задания

`DELETE /job/{id}/`

Удаление задания не происходит полностью, ему присваивается статус `deleted`.

Метод вернет `404: Job not found`, если задание по указанному `id` не найдено.

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

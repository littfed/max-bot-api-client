# Исследование: Форматирование постов, разметка и Rich Content в MAX Bot API

> **Статус документа:** Завершено (Verified via Live API Probes)  
> **Дата исследования:** Ноябрь 2026 г.  
> **Платформа:** MAX Messenger (VK / Mail.ru Group Enterprise), `platform-api2.max.ru`  
> **Методология:** Анализ исходного кода, официальной документации `dev.max.ru`, TamTam-наследия и экспериментальная верификация живыми вызовами к API с валидным бот-токеном.

---

## 1. Введение и ключевые выводы (Executive Summary)

В ходе живого экспериментального исследования взаимодействия с шлюзом `platform-api2.max.ru` были точно установлены и задокументированы правила форматирования текста, работы с медиавложениями, интерактивными клавиатурами и ограничениями платформы MAX Bot API.

### Главные открытия и особенности платформы:

1. **Серверная модель хранения разметки (AST / Markup Array):**
   При отправке форматированного сообщения (как Markdown, так и HTML) сервер парсит его и сохраняет в поле `body` два взаимосвязанных объекта:
   * `body.text`: очищенный plain text (без спецсимволов разметки `**`, `<i>` и т.д.);
   * `body.markup`: массив структур смещений `[{"from": <offset>, "length": <len>, "type": "<markup_type>", ...}]`.
   
2. **Markdown vs HTML:**
   * **Markdown (`format: "markdown"`):** поддерживает `**жирный**` (`strong`), `*курсив*` (`emphasized`), `~~зачеркнутый~~` (`strikethrough`), моноширинный код `` `code` `` и блоки кода ` ```code``` ` (`monospaced`), гиперссылки `[текст](url)` (`link`), и юзернеймы `@username` (`user_mention`).  
     *Подчеркивание (`<u>` или `+`) и спойлеры (`||текст||`) в Markdown НЕ поддерживаются!*
   * **HTML (`format: "html"`):** поддерживает `<b>`, `<strong>`, `<i>`, `<em>`, `<u>` (**подчеркивание!**), `<s>`, `<del>` (**зачеркивание**), `<code>`, `<pre>` (**моноширинный код**), `<mark>` (**подсветка/highlighted!**), и `<a href="...">`.  
     *Подчеркивание (`<u>`) и подсветка (`<mark>`) доступны ТОЛЬКО в режиме HTML!*

3. **Скрытый текст (Спойлеры):**
   **Спойлеры платформой НЕ поддерживаются.** Ни синтаксис `||текст||`, ни теги `<spoiler>`, `<tg-spoiler>`, `<span class="tg-spoiler">` сервером не распознаются: они либо сохраняются как экранированный сырой текст, либо очищаются. Типа разметки `spoiler` в схеме сервера нет.

4. **Ограничение длины текста:**
   Строго **4 000 символов UTF-8**. Отправка 4 001 символа возвращает ошибку `400 Bad Request` с сообщением `Field 'text' size (4001) must be at most 4000`.

5. **Клавиатуры (Inline Keyboards):**
   * Максимум кнопок в одном ряду: **7 кнопок** (8 кнопок вызывают `errors.maxRowSize`).
   * Максимум рядов: **30 рядов** (31 ряд вызывает `errors.maxRows`).
   * Максимум кнопок в клавиатуре: **210 кнопок** (30 × 7).
   * Поддерживаются **7 типов кнопок**: `callback` (с визуальными стилями `intent`: positive, negative, primary, default), `link`, `open_app`, `clipboard`, `request_contact`, `request_geo_location`, `message`.
   * Для кнопки `open_app` поле приложения называется `web_app` и принимает имя зарегистрированного приложения / юзернейм бота в MAX, а не внешний произвольный URL!
   * Кнопка `clipboard` требует обязательное поле `payload` (а не `data`).

6. **Связывание сообщений (Forward vs Reply):**
   * При ответе (`link: {"type": "reply", "mid": "..."}`) текст сообщения обязателен или опционален.
   * При пересылке (`link: {"type": "forward", "mid": "..."}`) текст **ОБЯЗАН БЫТЬ ПУСТЫМ** (`text: ""`). Пересылка с текстом отклоняется ошибкой `400 Bad Request: errors.forward.text.not-empty`.

7. **Вложения и превью:**
   * Ссылки в тексте автоматически создают сниппет (вложение типа `"share"`). Отключить превью можно параметром `"disable_link_preview": true`.
   * Сообщения поддерживают отправку **медиагрупп / альбомов** (несколько медиа в массиве `attachments`).
   * Вложение `location` содержит координаты на верхнем уровне объекта вложения (`latitude`, `longitude`), а не внутри `payload`.
   * Вложение `contact` содержит `vcf_info` или `contact_id` (при этом сервер возвращает профиль в `payload.max_info`).

---

## 2. Архитектура форматирования сообщений

### 2.1. Внутренняя репрезентация (AST / Markup)

Когда клиент запрашивает сообщение через `GET /messages/{messageId}` или получает его через Long Polling / Webhook (`update_type: "message_created"`), разметка возвращается не в виде исходного текста с символами Markdown, а в структурированном виде:

```json
{
  "body": {
    "mid": "mid.00000000085e47aa01a1c71d171b7de6",
    "seq": 117593993044000220,
    "text": "Заказ #404: доставлен клиенту",
    "markup": [
      {
        "from": 0,
        "length": 10,
        "type": "strong"
      },
      {
        "from": 12,
        "length": 9,
        "type": "emphasized"
      }
    ]
  }
}
```

#### Типы элементов в `body.markup`:

| `type` в JSON | Описание | Поддерживается в Markdown | Поддерживается в HTML |
| :--- | :--- | :---: | :---: |
| `strong` | Полужирный текст (Bold) | Да (`**...**`, `__...__`) | Да (`<b>`, `<strong>`) |
| `emphasized` | Курсивный текст (Italic) | Да (`*...*`, `_..._`) | Да (`<i>`, `<em>`) |
| `underline` | Подчеркнутый текст | **Нет** | Да (`<u>...</u>`) |
| `strikethrough` | Зачеркнутый текст | Да (`~~...~~`) | Да (`<s>...</s>`, `<del>...`) |
| `monospaced` | Моноширинный код | Да (`` `...` `` и ` ```...``` `) | Да (`<code>`, `<pre>`) |
| `highlighted` | Выделенный/подсвеченный текст | **Нет** | Да (`<mark>...</mark>`) |
| `link` | Гиперссылка (`url`: `"..."`) | Да (`[label](url)`) | Да (`<a href="...">`) |
| `user_mention` | Упоминание (`user_link`: `"..."`) | Да (`@username`) | Автоматически |

> [!IMPORTANT]
> **Различия между Markdown и HTML в MAX Bot API:**
> 1. Подчеркивание (`<u>`) и выделение (`<mark>`) доступны **только** при `format: "html"`.
> 2. В Markdown одинарная тильда `~текст~` **не распознается**; зачеркивание требует строго двойной тильды `~~текст~~`.
> 3. Тег `<strike>` в HTML игнорируется, следует использовать `<s>` или `<del>`.

---

### 2.2. Формат Markdown (`format: "markdown"`)

Шлюз MAX реализует кастомный CommonMark-подобный диалект парсера со следующими спецификациями:

#### 1. Жирный шрифт (Bold)
* Синтаксис: `**жирный текст**` или `__жирный текст__`.
* Результат в AST: `type: "strong"`.
* Пример:
  ```json
  {"text": "Привет, **Иван**!", "format": "markdown"}
  ```

#### 2. Курсив (Italic)
* Синтаксис: `*курсив*` или `_курсив_`.
* Результат в AST: `type: "emphasized"`.
* Пример:
  ```json
  {"text": "Статус: _в обработке_", "format": "markdown"}
  ```

#### 3. Вложенное форматирование (Bold + Italic)
* Синтаксис: `**_жирный курсив_**` или `__*жирный курсив*__`.
* Результат в AST: два элемента разметки с одинаковыми `from` и `length` (`strong` и `emphasized`).
* Пример:
  ```json
  {"text": "**_Внимание!_** Срочное уведомление", "format": "markdown"}
  ```

#### 4. Зачеркнутый текст (Strikethrough)
* Синтаксис: строго `~~зачеркнутый текст~~` (двойные тильды). Одинарная тильда `~текст~` остается в сыром виде!
* Результат в AST: `type: "strikethrough"`.
* Пример:
  ```json
  {"text": "Старая цена: ~~15 000 руб~~, новая: 12 000 руб", "format": "markdown"}
  ```

#### 5. Моноширинный текст и блоки кода (Monospace)
* Инлайн-код: `` `const status = 200` ``.
* Блок кода:
  ````markdown
  ```go
  func main() {
      fmt.Println("MAX Bot API")
  }
  ```
  ````
* Результат в AST: `type: "monospaced"`. Спецификатор языка подсветки (`go`, `json`, `bash`) на сервере отсекается, сохраняется чистое тело блока кода.

#### 6. Цитаты (Blockquotes)
* Синтаксис: `> Текст цитаты`.
* Поведение сервера: символ `>` отсекается, сохраняются переводы строк, но отдельный элемент `type: "blockquote"` в массив `markup` не добавляется (визуальный отступ формируется клиентом на основе структуры строк).

#### 7. Гиперссылки и диплинки
* Синтаксис: `[Текст ссылки](https://example.com)`.
* Результат в AST: в поле `body.text` подставляется `Текст ссылки`, а ссылка выносится в объект `markup`: `{"from": ..., "length": ..., "type": "link", "url": "https://example.com"}`.
* **Диплинки на профили пользователей:**
  - `[Профиль](https://max.ru/u/396393759)` парсится как валидная веб-ссылка `type: "link"`.
  - Внутренняя схема `[Пользователь](max://user/396393759)` парсером Markdown не распознается как валидный URL: метка `Пользователь` сохраняется, а `url` отбрасывается.

#### 8. Упоминания пользователей (@mentions)
* Синтаксис: `@username`.
* Результат в AST: `{"type": "user_mention", "user_link": "username"}`.

#### 9. Неподдерживаемые в Markdown конструкции
* **Подчеркнутый текст:** ни `__подчеркнутый__` (воспринимается как bold), ни `<u>текст</u>`, ни `+текст+` не создают подчеркивания.
* **Спойлеры:** `||скрытый текст||` сохраняется как буквальные символы `||...||`.

---

### 2.3. Формат HTML (`format: "html"`)

Режим HTML идеально подходит для генерации сообщений из шаблонизаторов и CMS, а также открывает эксклюзивные типы стилей.

#### Поддерживаемые теги:
| HTML Тег | Результат в `markup` | Назначение |
| :--- | :--- | :--- |
| `<b>...</b>`, `<strong>...</strong>` | `type: "strong"` | Полужирный текст |
| `<i>...</i>`, `<em>...</em>` | `type: "emphasized"` | Курсив |
| `<u>...</u>` | `type: "underline"` | **Подчеркнутый текст** (эксклюзив HTML) |
| `<s>...</s>`, `<del>...` | `type: "strikethrough"` | Зачеркнутый текст |
| `<code>...</code>` | `type: "monospaced"` | Инлайн моноширинный код |
| `<pre>...</pre>` | `type: "monospaced"` | Блок преформатированного текста/кода |
| `<mark>...</mark>` | `type: "highlighted"` | **Желтая подсветка текста** (эксклюзив HTML) |
| `<a href="...">...</a>` | `type: "link"`, `url` | Гиперссылка |
| `<blockquote>...</blockquote>` | теги удаляются | Цитата (plain text с переносами) |

#### Поведение парсера HTML при ошибках разметки:
1. **Незакрытые теги (Forgiving Parser):**
   Если бот отправляет незакрытый тег, например `<b>Важное сообщение без закрытия`, парсер MAX Bot API не выдает ошибку, а автоматически закрывает тег в конце строки (`EOF`), корректно вычисляя длину `length`.
2. **Синтаксически сломанные теги (Malformed Syntax):**
   Если отправлен оборванный угловой символ, например `<b текст без закрывающей скобки`, сервер возвращает `400 Bad Request`:
   ```json
   {
     "code": "proto.payload",
     "message": "errors.required"
   }
   ```
3. **Неизвестные теги (`<div>`, `<span>`, `<article>`):**
   Теги вырезаются (strip), текстовое содержимое сохраняется.

---

### 2.4. Исследование скрытого текста (Спойлеры / Spoilers)

Один из частых запросов разработчиков — поддержка скрытого текста под плашкой (спойлер). Была проведена исчерпывающая серия тестов с различными форматами разметки:

| Проверяемый синтаксис | Формат запроса | Результат в API | Как сохранено на сервере |
| :--- | :--- | :---: | :--- |
| `\|\|секретный текст\|\|` | `markdown` | `200 OK` | Буквальный текст: `\|\|секретный текст\|\|` (`markup: []`) |
| `<spoiler>секрет</spoiler>` | `markdown` | `200 OK` | Буквальный текст: `\u003cspoiler\u003e...` |
| `<tg-spoiler>секрет</tg-spoiler>` | `markdown` | `200 OK` | Буквальный текст: `\u003ctg-spoiler\u003e...` |
| `<spoiler>секрет</spoiler>` | `html` | `200 OK` | Тег вырезан, текст `секрет` (`markup: []`) |
| `<tg-spoiler>секрет</tg-spoiler>` | `html` | `200 OK` | Тег вырезан, текст `секрет` (`markup: []`) |
| `<span class="tg-spoiler">...</span>` | `html` | `200 OK` | Тег вырезан, текст сохранен (`markup: []`) |

> [!WARNING]
> **Вывод по спойлерам:**
> В настоящее время MAX Bot API **не поддерживает функционал спойлеров**. Попытка отправки спойлеров в Markdown оставляет символы `||` видимыми в тексте, а в HTML просто отсекает теги без создания интерактивного скрытия.

---

## 3. Вложения и медиаконтент (Attachments)

Сообщение в MAX Bot API может содержать гетерогенный массив вложений `attachments`.

### 3.1. Двухфазная загрузка медиафайлов

Загрузка любых файлов в MAX осуществляется в два этапа:
1. Запрос пресайнд-URL: `POST https://platform-api2.max.ru/uploads?type={image|video|audio|file}` с заголовком `Authorization: <token>`.
2. Загрузка бинарных данных: `POST` на полученный `url` с типом `multipart/form-data` (поле `file`).
3. Получение токена вложения:
   - Для изображений возвращается структура вида: `{"photos": {"<hash>": {"token": "<attachment_token>"}}}`.
   - Для файлов, видео и аудио возвращается: `{"token": "<attachment_token>"}`.

```
+-------------+                     +----------------------+                     +---------------------+
|   Go Bot    |                     | platform-api2.max.ru |                     |  Upload / Transcode |
+-------------+                     +----------------------+                     +---------------------+
       |                                       |                                            |
       | 1. POST /uploads?type=image           |                                            |
       |-------------------------------------->|                                            |
       |                                       |                                            |
       | 2. 200 OK {url, token}                |                                            |
       |<--------------------------------------|                                            |
       |                                                                                    |
       | 3. POST multipart/form-data (file: binary)                                         |
       |----------------------------------------------------------------------------------->|
       |                                                                                    |
       | 4. 200 OK {"photos": {"0": {"token": "UPLOAD_TOKEN"}}}                            |
       |<-----------------------------------------------------------------------------------|
       |                                       |                                            |
       | 5. POST /messages (attachments: [token])                                           |
       |-------------------------------------->|                                            |
       |                                       |                                            |
       | 6. 200 OK (Message sent)              |                                            |
       |<--------------------------------------|                                            |
```

> [!NOTE]
> **Асинхронная обработка (`attachment.not.ready`):**
> Для тяжелых файлов и видео сервер производит антивирусную проверку и транскодирование. Если бот пытается отправить сообщение слишком быстро, сервер возвращает статус `400 Bad Request` с кодом `attachment.not.ready`:
> ```json
> {
>   "code": "attachment.not.ready",
>   "message": "Key: errors.process.attachment.file.not.processed"
> }
> ```
> Клиентская библиотека должна реализовывать политику повторных попыток (Retry Policy) с задержкой 500–1000 мс.

---

### 3.2. Медиагруппы и альбомы (Multi-Attachment)

В отличие от Telegram Bot API, где для альбомов требуется отдельный эндпоинт `sendMediaGroup`, в MAX Bot API отправка нескольких медиафайлов происходит **нативно в рамках одного вызова `POST /messages`**:

```json
{
  "text": "Отчет и фотоматериалы объекта",
  "attachments": [
    {
      "type": "image",
      "payload": {
        "token": "TOKEN_IMAGE_1"
      }
    },
    {
      "type": "image",
      "payload": {
        "token": "TOKEN_IMAGE_2"
      }
    },
    {
      "type": "file",
      "payload": {
        "token": "TOKEN_PDF_DOC"
      }
    },
    {
      "type": "inline_keyboard",
      "payload": {
        "buttons": [[{"type": "callback", "text": "Подтвердить", "payload": "ok"}]]
      }
    }
  ]
}
```

* Платформа разрешает комбинацию нескольких медиа, документов и инлайн-клавиатуры в одном сообщении.
* Поле `text` может быть пустым (`""`), если в сообщении присутствует хотя бы одно вложение (`attachment`).

---

### 3.3. Вложения Location и Contact: критические нюансы схемы

Экспериментальная проверка выявила важные расхождения между теоретическими описаниями и реальным поведением шлюза MAX:

#### Вложение геопозиции (`type: "location"`):
Координаты должны передаваться **на верхнем уровне объекта вложения**, а не вложенными в `payload`:
```json
{
  "type": "location",
  "latitude": 55.7558,
  "longitude": 37.6173
}
```
*Если передать координаты внутри `payload`, сервер вернет ошибку `Field 'latitude' cannot be null`.*

#### Вложение контакта (`type: "contact"`):
Контакт передается внутри объекта `payload` и поддерживает два сценария:
1. **Передача произвольной карточки через VCF:**
   ```json
   {
     "type": "contact",
     "payload": {
       "vcf_info": "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Иван Иванов\r\nTEL:+79991234567\r\nEND:VCARD"
     }
   }
   ```
2. **Передача существующего пользователя MAX по его ID (`contact_id`):**
   ```json
   {
     "type": "contact",
     "payload": {
       "contact_id": 396393759
     }
   }
   ```
   В этом случае сервер автоматически резолвит профиль и возвращает вложение с объектом `max_info`:
   ```json
   {
     "type": "contact",
     "payload": {
       "max_info": {
         "user_id": 396393759,
         "first_name": "Федор",
         "name": "Федор В"
       }
     }
   }
   ```

---

### 3.4. Превью ссылок (Link Previews) и сниппеты

1. **Автоматическая генерация:** Если в тексте сообщения присутствует URL, сервер автоматически подгружает OpenGraph-метаданные целевого сайта (заголовок, описание, изображение) и сохраняет их в `body.attachments` в виде вложения с типом `"share"`:
   ```json
   {
     "type": "share",
     "title": "Яндекс",
     "description": "Найдётся всё",
     "image_url": "https://i.oneme.ru/i?r=...",
     "payload": {
       "url": "https://ya.ru",
       "token": "..."
     }
   }
   ```
2. **Отключение превью:** Если генерация карточки нежелательна, бот передает в запросе флаг:
   ```json
   "disable_link_preview": true
   ```

---

### 3.5. Связывание сообщений (Forward и Reply через `link`)

Связи между сообщениями конфигурируются через объект `link` в теле запроса:

#### 1. Ответ на сообщение (Reply)
Используется для контекстных ответов в тредах и чатах:
```json
{
  "text": "Ваш запрос принят в обработку",
  "link": {
    "type": "reply",
    "mid": "mid.00000000085e47aa01a1c71d642f3b82"
  }
}
```

#### 2. Пересылка сообщения (Forward)
Используется для цитирования исходного сообщения:
```json
{
  "text": "",
  "link": {
    "type": "forward",
    "mid": "mid.00000000085e47aa01a1c71d642f3b82"
  }
}
```

> [!CAUTION]
> **Строгое правило сервера для Forward:**
> При пересылке сообщения поле `text` **обязано быть пустой строкой** (`""`). Если передать любой непустой текст, запрос будет отклонен с ошибкой `400 Bad Request: {"code": "proto.payload", "message": "errors.forward.text.not-empty"}`.

---

## 4. Интерактивные клавиатуры (Inline Keyboards)

Инлайн-клавиатура передается как вложение типа `"inline_keyboard"` и содержит двумерный массив кнопок: `buttons: Button[][]`.

### 4.1. Все поддерживаемые типы кнопок

| Тип кнопки (`type`) | Обязательные поля | Опциональные поля | Описание |
| :--- | :--- | :--- | :--- |
| `callback` | `text`, `payload` | `intent` (`positive`, `negative`, `primary`, `default`) | Генерирует событие `message_callback` при клике. Поддерживает цветные стили. |
| `link` | `text`, `url` | — | Открывает внешнюю веб-ссылку в браузере. |
| `open_app` | `text`, `web_app` | — | Запускает MAX Mini App внутри встроенного WebView. `web_app` — юзернейм бота или слаг сервиса. |
| `clipboard` | `text`, `payload` | — | В один клик копирует переданный текст из `payload` в буфер обмена операционной системы. |
| `request_contact` | `text` | — | Запрашивает у пользователя подтверждение на отправку его верифицированного номера телефона. |
| `request_geo_location` | `text` | `quick` (bool) | Запрашивает отправку текущих координат GPS. Если `quick: true`, отправляет геопозицию без доп. диалога. |
| `message` | `text` | — | Отправляет заданный текст в чат от имени пользователя при нажатии. |

#### Стилизация кнопок (`intent`):
Кнопки типа `callback` поддерживают визуальную цветовую дифференциацию в клиентах MAX:
* `positive` — Зеленый акцент (успех, подтверждение, оплата);
* `negative` — Красный акцент (отмена, удаление, отклонение);
* `primary` — Синий / акцентный стиль бренда (главное действие);
* `default` — Стандартный серый контурный фон.

---

### 4.2. Точные лимиты размеров клавиатуры

Экспериментальным путем установлены граничные лимиты валидатора MAX API:

| Параметр клавиатуры | Лимит платформы | Код ошибки при превышении |
| :--- | :---: | :--- |
| **Максимум кнопок в одном ряду** | **7** | `400 Bad Request: errors.maxRowSize` |
| **Максимум рядов в сообщении** | **30** | `400 Bad Request: errors.maxRows` |
| **Максимальное число кнопок** | **210** | — (30 рядов × 7 кнопок) |

> [!TIP]
> **Рекомендация по мобильному UX:**
> Хотя протокол допускает до 7 кнопок в ряду, на экранах смартфонов оптимально размещать не более 3–4 кнопок стандартного размера и не более 1–2 кнопок для действий с длинными названиями или специальных кнопок (`request_contact`, `request_geo_location`).

---

## 5. Сводная таблица лимитов платформы (Platform Limits)

| Ресурс / Параметр | Значение лимита | Поведение платформы при превышении |
| :--- | :--- | :--- |
| **Длина текста сообщения** | **4 000 символов UTF-8** | `400 Bad Request`: `Field 'text' size (N) must be at most 4000` |
| **Размер изображения (image)** | До **50 МБ**, до 7680×7680 px | Ошибка загрузки на этапе multipart POST |
| **Размер видео (video)** | До **250 МБ** (MP4, MOV) | Ошибка загрузки |
| **Размер аудио/голоса (audio)**| До **100 МБ** (OGG, MP3, WAV) | Ошибка загрузки |
| **Размер файлов (file)** | До **4 ГБ** | Ошибка загрузки |
| **Кнопок в ряду** | До **7 кнопок** | `errors.maxRowSize` |
| **Рядов кнопок** | До **30 рядов** | `errors.maxRows` |
| **Всего кнопок** | До **210 кнопок** | `errors.maxRows` / `errors.maxRowSize` |
| **Количество команд бота** | До **32 команд** (`PATCH /me/commands`) | Отклонение запроса |
| **Тихие сообщения (`notify: false`)** | Доступны в диалогах и группах | В каналах `notify: false` может быть отклонен |
| **Срок редактирования сообщений** | До 7 дней в ЛС; бессрочно в группах и при наличии клавиатуры | Ошибка редактирования истекшего сообщения |

---

## 6. Примеры запросов и реализации на Go

### 6.1. Markdown-сообщение с инлайн-клавиатурой (JSON)

```http
POST https://platform-api2.max.ru/messages?chat_id=140396458 HTTP/1.1
Authorization: f9LHodD0cOJattdbnvtca0kygnVvprJ1W5PSNYSZh1LCVnjLr...
Content-Type: application/json

{
  "text": "🎉 **Добро пожаловать в сервис доставки!**\n\nСтатус заказа `#8821`: _передан курьеру_.\n~~Старая стоимость: 1200 ₽~~ -> **Итого: 990 ₽**\n\nПодробнее на сайте: [Правила доставки](https://example.com/rules)",
  "format": "markdown",
  "disable_link_preview": true,
  "attachments": [
    {
      "type": "inline_keyboard",
      "payload": {
        "buttons": [
          [
            {
              "type": "callback",
              "text": "✅ Подтвердить",
              "payload": "order_confirm_8821",
              "intent": "positive"
            },
            {
              "type": "callback",
              "text": "❌ Отменить",
              "payload": "order_cancel_8821",
              "intent": "negative"
            }
          ],
          [
            {
              "type": "clipboard",
              "text": "📋 Скопировать трек-номер",
              "payload": "TRACK-8821-MAX"
            }
          ],
          [
            {
              "type": "request_contact",
              "text": "📱 Передать телефон курьеру"
            },
            {
              "type": "request_geo_location",
              "text": "📍 Мое местоположение",
              "quick": true
            }
          ]
        ]
      }
    }
  ]
}
```

---

### 6.2. HTML-сообщение с подчеркиванием и подсветкой (Go)

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func SendRichHTMLReport(ctx context.Context, client *maxbot.Client, chatID int64) error {
	htmlText := "<b>ВНИМАНИЕ!</b> На сервере зафиксирован инцидент.<br>" +
		"Критичность: <mark>HIGH SEVERITY</mark><br>" +
		"Действие: <u>Немедленно выполните проверку конфигурации</u>.<br>" +
		"Команда для проверки: <code>systemctl status bot.service</code><br>" +
		"Подробнее: <a href=\"https://status.example.com\">Dashboard Инцидентов</a>"

	req := &maxbot.SendMessageRequest{
		Text:   htmlText,
		Format: "html",
	}

	msg, err := client.SendMessage(ctx, maxbot.ToChat(chatID), req)
	if err != nil {
		return fmt.Errorf("failed to send rich HTML message: %w", err)
	}

	log.Printf("Rich HTML message sent successfully, MID: %s", msg.ID())
	return nil
}
```

---

### 6.3. Отправка мультимедийного альбома (Image + Document + Keyboard) на Go

```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

func SendMediaAlbumWithKeyboard(ctx context.Context, client *maxbot.Client, chatID int64) error {
	// 1. Загрузка фотоснимка
	photoData, _ := os.ReadFile("receipt.png")
	photoUpload, err := client.UploadMedia(ctx, maxbot.UploadTypeImage, "receipt.png", bytes.NewReader(photoData))
	if err != nil {
		return fmt.Errorf("failed to upload receipt photo: %w", err)
	}

	// 2. Загрузка PDF отчета
	pdfData, _ := os.ReadFile("invoice.pdf")
	pdfUpload, err := client.UploadMedia(ctx, maxbot.UploadTypeFile, "invoice.pdf", bytes.NewReader(pdfData))
	if err != nil {
		return fmt.Errorf("failed to upload invoice pdf: %w", err)
	}

	// 3. Формирование интерактивной клавиатуры
	kb := maxbot.NewInlineKeyboard().
		Row(
			maxbot.NewIntentCallbackButton("Подписать акт", "sign_act", maxbot.IntentPositive),
			maxbot.NewIntentCallbackButton("Запросить правки", "request_edits", maxbot.IntentNegative),
		)

	// 4. Сборка комплексного сообщения (Альбом)
	msgReq := &maxbot.SendMessageRequest{
		Text:   "🧾 **Пакет документов по сделке готов к согласованию.**",
		Format: "markdown",
		Attachments: []maxbot.Attachment{
			{
				Type: "image",
				Payload: map[string]string{"token": photoUpload.Token},
			},
			{
				Type: "file",
				Payload: map[string]string{"token": pdfUpload.Token},
			},
			kb.ToAttachment(),
		},
	}

	_, err = client.SendMessage(ctx, maxbot.ToChat(chatID), msgReq)
	return err
}
```

---

### 6.4. Безопасная пересылка сообщения (Forwarding) на Go

```go
package main

import (
	"context"
	"fmt"

	"github.com/littfed/max-bot-api-client/pkg/maxbot"
)

// ForwardMessage безопасно пересылает сообщение с гарантией пустого поля text.
func ForwardMessage(ctx context.Context, client *maxbot.Client, recipient maxbot.Recipient, sourceMID string) (*maxbot.Message, error) {
	req := &maxbot.SendMessageRequest{
		Text: "", // Сервер MAX строго требует пустой текст при type=forward
		Link: &maxbot.MessageLink{
			Type: "forward",
			Message: &maxbot.Message{
				MessageID: sourceMID,
			},
		},
	}
	return client.SendMessage(ctx, recipient, req)
}
```

---

## 7. Рекомендации по развитию библиотеки `max-bot-api-client`

По результатам исследования сформирован список улучшений для кодовой базы Go SDK:

1. **Добавление методов для HTML-форматирования:**
   В клиенте `pkg/maxbot/client.go` добавить высокоуровневый метод `SendHTMLMessage(ctx, recipient, htmlText)` наряду с существующим `SendTextMessage`.

2. **Коррекция сериализации моделей вложений:**
   * В структуре `LocationAttachmentPayload`: убедиться, что поля `latitude` и `longitude` сериализуются на верхний уровень объекта `Attachment`, если формируется исходящий запрос геолокации.
   * В структуре `Button` (`keyboards.go`): поле `webApp` для кнопки `open_app` должно иметь JSON-тег `json:"web_app,omitempty"`, так как сервер ищет слаг приложения именно в `web_app`.
   * Поле `payload` для кнопки `clipboard` должно быть строго заполнено строкой для копирования.

3. **Валидация клавиатуры на этапе компиляции/построения (Fail-fast):**
   В построителе `InlineKeyboard` добавить проверки:
   * Предупреждение или ошибка, если в ряд добавляется более 7 кнопок (`Row(...)` с `len > 7`).
   * Ошибка при добавлении более 30 рядов.

4. **Защита от ошибки `errors.forward.text.not-empty`:**
   В методе `ForwardMessage` или при сериализации `SendMessageRequest` автоматически очищать поле `Text` в пустую строку `""`, если `Link.Type == "forward"`.

5. **Обработка `attachment.not.ready` при загрузке файлов:**
   В хелперах загрузки файлов добавить автоматический ретрай с экспоненциальной задержкой (например, 3 попытки с интервалом 500ms, 1000ms, 2000ms), если сервер вернул `errors.process.attachment.file.not.processed`.

# Google Calendar: настройка и права доступа

Этот документ описывает настройку Google Calendar для бота, процесс авторизации и управление правами (reader/writer).

## 1. Подготовка Google Cloud проекта

1. В Google Cloud Console включите **Google Calendar API** для проекта.
2. Настройте **OAuth consent screen** (брендинг, аудитория, data access).
3. Создайте **OAuth Client ID** (тип: Web application).
4. Добавьте **Authorized redirect URI**:
   - Значение должно **полностью совпадать** с `https://<redirect_domain><url>` из `configs/config.yaml`.
   - Пример: `https://mgke.example.com/google/oauth`.
   - Для веб‑приложений используется HTTPS (локально допустим http://localhost).

> Если redirect URI не совпадает (вплоть до https/слэшей), Google вернёт ошибку `redirect_uri_mismatch`.

## 2. Конфигурация в проекте

В `configs/config.yaml` заполните:

```yaml
google:
  redirect_domain: "https://mgke.example.com"
  url: "/google/oauth"
  oauth:
    client_id: "...apps.googleusercontent.com"
    client_secret: "..."
  service_account:
    client_email: "calendar@project.iam.gserviceaccount.com"
    private_key: "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----\n"
  calendar_owners: []
```

## 3. Привязка аккаунта пользователем

1. В Telegram: команда `/calendar` → выбрать **Google**.
2. Нажать кнопку "Привязать Google аккаунт".
3. Пройти OAuth и вернуться в чат.

После этого бот сохранит email пользователя и сможет добавлять календарь в его аккаунт.

## 4. Добавление календаря

В меню Google выберите **Добавить**:
- Бот создаст календарь расписания (если его ещё нет).
- Добавит календарь в ваш аккаунт.

## 5. Права доступа (reader / writer)

По умолчанию календарь добавляется с правами **reader**.
Чтобы временно разрешить редактирование:

1. В меню Google нажмите **Права**.
2. Выберите нужный календарь.
3. Нажмите **Дать права редактирования** (writer).

Чтобы вернуть режим чтения:

1. Нажмите **Снять права редактирования** (reader).

### Что означают роли
- `reader`: только просмотр.
- `writer`: просмотр и редактирование.
- `owner`: управление ACL (не используем для пользователей; у календаря один data owner).

## 6. Частые ошибки

### redirect_uri_mismatch
Причина: redirect URI в Google Cloud не совпадает с `redirect_domain + url` в конфиге.
Решение: исправьте URI в Google Cloud Console или в конфиге.

### Недостаточно прав
Причина: аккаунт не авторизован или не выдан writer.
Решение: пройти OAuth и дать права через меню "Права".

## Ссылки на официальные документы
- OAuth redirect URI и правила совпадения: [Google OAuth 2.0 (Web Server)](https://developers.google.com/identity/protocols/oauth2/web-server)
- Настройка OAuth consent screen: [Google Calendar API Auth Guide](https://developers.google.com/calendar/api/guides/auth)
- Роли ACL (reader/writer/owner): [Calendar API ACL](https://developers.google.com/workspace/calendar/api/v3/reference/acl)

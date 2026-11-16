# Мониторинг бота расписаний МГКЦТ

[English version](README.md)

Готовый набор мониторинга для ручки Prometheus бота (`metrics.enabled`,
`metrics.path`, `metrics.token` — см. [основной README](../../README.md#prometheus-metrics)).
Ничего здесь не нужно править до запуска: стек compose сам записывает адрес
скрейпа и bearer-токен, дашборд ставится одним импортом, а правила грузятся и
обычным Prometheus, и Prometheus Operator, и Grafana Cloud.

Всё офлайн и проверяемо: файлы — только конфигурация (никаких вложенных
бинарников, никакого Grizzly, никакого Grafonnet, никакого Grafana Managed
Alerts JSON), а `tests/monitoring_test.go` падает всякий раз, когда правило,
панель дашборда или манифест Kubernetes упоминает имя метрики, которого
`api.MetricFamilies()` не объявляет, или когда копии расходятся между собой.

Текст здесь намеренно на английском: репозиторий держит русский
пользовательский текст только в `internal/i18n/locales/ru.json`, поэтому эти
операторские ассеты остаются латиницей.

## Что в наборе

| Путь | Что это |
|------|---------|
| `prometheus/prometheus.yml` | конфиг скрейпа: одна джоба `mgke`, bearer-токен из смонтированного файла, адрес скрейпа из file_sd |
| `prometheus/rules/mgke.yml` | 14 правил алертов в пяти группах (доступность, парсер, календарь, api, telegram) |
| `prometheus/rules/mgke.test.yml` | unit-тесты promtool для этих правил |
| `alertmanager/alertmanager.yml` | профиль Alertmanager с пустыми получателями, чтобы алерты оставались видны и ничего не уходило случайно |
| `grafana/dashboards/mgke-timetablebot.json` | дашборд, выгруженный в модели импорта Grafana (`__inputs`, `DS_PROMETHEUS`) |
| `grafana/provisioning/` | провайдеры datasource и дашбордов для стека compose |
| `kubernetes/servicemonitor.yaml` | референсный Service плюс ServiceMonitor `monitoring.coreos.com/v1` |
| `kubernetes/prometheusrule.yaml` | те же группы правил в виде PrometheusRule `monitoring.coreos.com/v1` |
| `kubernetes/grafana-dashboard-configmap.yaml` | тот же дашборд в виде ConfigMap с меткой `grafana_dashboard: "1"` для sidecar Grafana |
| `alloy/config.alloy` | конфиг скрейпа и remote_write Grafana Alloy для случая Grafana Cloud |
| `../../docker-compose.monitoring.yml` | сам стек: Prometheus, Grafana и, под профилем, Alertmanager |

## Быстрый старт с docker compose

Стек скрейпит бота, запущенного на хосте. Единственное, что ему нужно, — адрес
метрик бота, а также токен, когда задан `metrics.token`:

```bash
export MGKE_METRICS_TARGET=host.docker.internal:8081   # api.port of the bot
export MGKE_METRICS_TOKEN='<metrics.token, leave unset when the route is open>'
export GRAFANA_ADMIN_PASSWORD='<a password you choose>'

docker compose -f docker-compose.monitoring.yml up -d
```

| Где | Что |
|-----|-----|
| <http://localhost:9090/targets> | таргет `mgke`, `UP` как только скрейп заработал |
| <http://localhost:9090/alerts> | правила из `prometheus/rules/mgke.yml`, активные и в ожидании |
| <http://localhost:3000> | Grafana; дашборд **MGKE Timetable Bot** уже провижинится в папке *MGKE* |

Вместе с Alertmanager (алерты видны на <http://localhost:9093> и не уходят
никуда, пока не настроен получатель):

```bash
docker compose -f docker-compose.monitoring.yml --profile alerts up -d
```

Перезагрузить правила после правки, без перезапуска:

```bash
curl -s -X POST http://127.0.0.1:9090/-/reload
```

Остановить стек, сохранив данные: `docker compose -f docker-compose.monitoring.yml down`.
Добавляйте `-v` только когда должны уйти и сохранённые сэмплы, и база Grafana.

## Ручной импорт дашборда

Grafana умеет принять дашборд и обычным импортом файла — тогда она спрашивает
datasource прямо при импорте:

1. Grafana → **Dashboards** → **New** → **Import**.
2. Загрузите `deploy/monitoring/grafana/dashboards/mgke-timetablebot.json`.
3. Выберите datasource Prometheus, когда спросят про `DS_PROMETHEUS`, и импортируйте.

Выгруженная модель — это `schemaVersion: 39` с записью `__inputs` под именем
`DS_PROMETHEUS`; Grafana мигрирует её вперёд при импорте, и дашборд продолжает
работать после переименования datasource, потому что каждая панель ссылается на
`${DS_PROMETHEUS}`, а не на жёстко прописанный uid.

## Загрузка правил

В работающий Prometheus — смонтировав `prometheus/rules/mgke.yml` и перечислив
его в `rule_files`:

```yaml
rule_files:
  - /etc/prometheus/rules/mgke.yml
```

Проверка и unit-тесты — по одному запуску promtool на каждую задачу:

```bash
promtool check rules deploy/monitoring/prometheus/rules/mgke.yml
promtool test rules deploy/monitoring/prometheus/rules/mgke.test.yml
```

Из-за этих unit-тестов правила парсера выглядят именно так. Бот, который ни
разу ничего не распарсил, вообще не сообщает об успешном парсинге, и значение
`mgke_parser_lag_seconds` в этом случае читается как ноль — «ноль секунд
отставания» это не здоровое расписание, а парсинг, которого не было. Поэтому
правила никогда не смотрят на лаг в одиночку: правило устаревания соединяет
гейдж лага с гейджем свежести долговременного кэша
(`mgke_cache_groups_last_update_seconds > 0 and (… freshness > 15m or lag > 15m)`),
а случай «ни разу не заполнен» — это отдельное правило на `mgke_uptime_seconds`
плюс `mgke_cache_groups_last_update_seconds == 0`, чего правило только про лаг
увидеть не может. `mgke.test.yml` закрепляет все три ситуации: кэш, который ни
разу не заполнялся, заполненный кэш, который устарел, и свежий кэш, у которого
устарела запись о парсинге.

Аннотации обходятся без шаблонов, чтобы те же самые строки можно было сверять
дословно в `mgke.test.yml` (promtool раскрывает `{{ ... }}` в
`exp_annotations`, так что статичная строка — единственная форма, которая
сравнивается равной где угодно при чтении).

## Алерты

| Алерт | Важность | Срабатывает, когда |
|-------|----------|--------------------|
| [`MgkeMetricsGone`](#mgkemetricsgone) | critical | ручка метрик не отвечает вообще ничем |
| [`MgkeHealthAlertsActive`](#mgkehealthalertsactive) | warning | собственные ключи алертов бота остаются активными |
| [`MgkeParserNeverFilled`](#mgkeparserneverfilled) | critical | кэш так и не заполнялся, хотя бот работает |
| [`MgkeParserStale`](#mgkeparserstale) | warning | ни кэш, ни запись о парсинге не двигались 15 минут |
| [`MgkeParserFrozen`](#mgkeparserfrozen) | critical | данным расписания больше шести часов |
| [`MgkeParserFailing`](#mgkeparserfailing) | warning | два или больше прогона парсинга подряд завершились ошибкой |
| [`MgkeParserLayout`](#mgkeparserlayout) | warning | обязательный селектор страницы перестал совпадать |
| [`MgkeParserGuard`](#mgkeparguard) | critical | защита от потери данных оставила предыдущий кэш |
| [`MgkeCalendarFailing`](#mgkecalendarfailing) | warning | две или больше синхронизации календаря подряд завершились ошибкой |
| [`MgkeCalendarStale`](#mgkecalendarstale) | warning | ни одной успешной синхронизации календаря за шесть часов |
| [`MgkeAPIErrorBurst`](#mgkeapierrorburst) | warning | три ответа 5xx за десять минут |
| [`MgkeAPIErrorRate`](#mgkeapierrorrate) | critical | больше 2 % из минимум 20 запросов за 20 минут падают |
| [`MgkeAPISlowRoute`](#mgkeapislowroute) | warning | маршрут в среднем медленнее двух секунд |
| [`MgkeSendFailures`](#mgkesendfailures) | warning | транспорт Bot API возвращает ошибки |

### MgkeMetricsGone

`absent(mgke_uptime_seconds)` в течение пяти минут: бот упал, ручка метрик была
выключена, или неверен адрес скрейпа либо токен. Проверьте контейнер,
`metrics.enabled` и таргет в `/etc/prometheus/targets`, затем
`docker compose -f docker-compose.monitoring.yml up -d scrape-config`.

### MgkeHealthAlertsActive

Собственный слой здоровья бота недоволен: телеграмный алерт о здоровье уже
ушёл на `telegram.admin_ids`. Команда `/incidents` перечисляет ключи алертов,
падающие эндпоинты API и последние 20 инцидентов, `/parserhealth` показывает
состояние парсера, а `/webhook` — состояние вебхука.

### MgkeParserNeverFilled

Бот работает больше 30 минут, а `mgke_cache_groups_last_update_seconds` всё
ещё ноль, то есть ни один парсинг не удался. Лаг парсера здесь читается как
ноль и выглядит здоровым — это правило и есть причина, по которой существует
гейдж свежести. Проверьте лог парсера и живой сайт.

### MgkeParserStale

Кэш заполнялся хотя бы раз, но ни отдаваемый кэш, ни запись бота о парсинге не
двигались 15 минут. Бот тем временем отдаёт последнюю хорошую копию расписания,
так что это раннее предупреждение; сверьтесь с ошибками парсинга и проверками
вёрстки на дашборде.

### MgkeParserFrozen

Шесть часов без успешного парсинга: каждый чат получает вчерашнее расписание.
Разберитесь с сайтом, логом парсера и защитами до того, как следующий парсинг
перезапишет запись кэша.

### MgkeParserFailing

Два или больше прогона парсинга подряд завершились ошибкой. Кэш сохраняет
предыдущую запись, поэтому расписание остаётся доступным, но устаревшим. Лог
парсера называет падающий источник и переставшие совпадать проверки.

### MgkeParserLayout

Последний парсинг сообщил об отсутствующих селекторах: сайт колледжа поменял
разметку. Каждая обязательная проверка называет ожидаемый CSS-селектор, так что
лог парсера указывает прямо на сдвинувшийся блок.

### MgkeParserGuard

Сработала защита от потери данных: парсинг вернул заметно меньше данных, чем
раньше, и предыдущая запись кэша была сохранена. Расписание устаревшее, а не
пустое; проверьте сайт до следующего прогона парсинга.

### MgkeCalendarFailing

Две или больше синхронизации календаря подряд завершились ошибкой. Неудачные
дни повторяются, потому что маркер ручной синхронизации двигается только после
успешного прохода; проверьте сервисный аккаунт, гугл-аккаунт конкретного чата и
квоту календаря.

### MgkeCalendarStale

Синхронизация запускалась хотя бы раз, но не завершалась успешно шесть часов.
Проход сверки добирает пропущенные дни, как только одна синхронизация
сработает, а кнопка `calendarSync` на алерте о здоровье запускает её вручную.

### MgkeAPIErrorBurst

Три или больше ответа 5xx за десять минут, с `keep_firing_for: 10m`, чтобы
дёргающийся эндпоинт не разрешался и не срабатывал заново на каждой проверке.
Алерт бота `api_errors` называет падающие эндпоинты; дашборд показывает, какой
маршрут начал отвечать 5xx.

### MgkeAPIErrorRate

Минимум 20 запросов за 20 минут и больше 2 % из них — 5xx. Сравните панель
ошибок по маршрутам с панелью самого медленного маршрута, чтобы найти
деградировавший обработчик, и проверьте `health.api_slow_ms`, который управляет
пометкой внутри бота.

### MgkeAPISlowRoute

Маршрут в среднем медленнее двух секунд в течение 15 минут. Метки алерта
несут HTTP-метод и путь, поэтому первая панель, куда идти, — «Average response
time per route».

### MgkeSendFailures

Транспорт Bot API возвращал ошибки последние 20 минут, поэтому уведомления или
ответы не дошли до своих чатов. Метка называет транспорт; и заблокировавшие
бота пользователи, и отозванный токен видны здесь.

## Kubernetes

```bash
kubectl apply -f deploy/monitoring/kubernetes/servicemonitor.yaml
kubectl apply -f deploy/monitoring/kubernetes/prometheusrule.yaml
kubectl apply -f deploy/monitoring/kubernetes/grafana-dashboard-configmap.yaml
```

| Манифест | Как он попадает в Prometheus или Grafana |
|----------|------------------------------------------|
| `servicemonitor.yaml` | референсный Service плюс `ServiceMonitor`; Prometheus из kube-prometheus-stack находит его по метке `app.kubernetes.io/name` |
| `prometheusrule.yaml` | `PrometheusRule`, который оператор грузит как любой другой файл правил |
| `grafana-dashboard-configmap.yaml` | ConfigMap с меткой `grafana_dashboard: "1"`; sidecar Grafana копирует его в Grafana, а дашборд разрешает `DS_PROMETHEUS` через собственную переменную datasource (uid по умолчанию `prometheus`) |

Держите в согласии с вашим Deployment три вещи: метку пода
`app.kubernetes.io/name: mgke-timetablebot`, порт контейнера с именем `http` и
секрет `mgke-metrics-token` с ключом `token`, когда задан `metrics.token`.
Когда ручка открыта, ServiceMonitor этого не требует — в таком случае уберите
блок `authorization`.

PrometheusRule сгенерирован из обычного файла правил; `tests/monitoring_test.go`
сравнивает их и печатает команду перегенерации, когда они расходятся.

## Grafana Cloud

`alloy/config.alloy` скрейпит бота и отправляет сэмплы в инстанс метрик Grafana
Cloud. Учётные данные приходят из окружения, никогда из файла:

```bash
docker run --rm \
  -e MGKE_METRICS_TARGET=bot:8081 \
  -e MGKE_METRICS_TOKEN="$MGKE_METRICS_TOKEN" \
  -e GRAFANA_CLOUD_PROMETHEUS_URL="$GRAFANA_CLOUD_PROMETHEUS_URL" \
  -e GRAFANA_CLOUD_PROMETHEUS_USERNAME="$GRAFANA_CLOUD_PROMETHEUS_USERNAME" \
  -e GRAFANA_CLOUD_API_KEY="$GRAFANA_CLOUD_API_KEY" \
  -v "$PWD/deploy/monitoring/alloy:/etc/alloy" \
  grafana/alloy:v1.20.1 validate /etc/alloy/config.alloy
```

Уберите `validate`, чтобы запустить его. Тот же файл правил грузится в Grafana
Cloud через rules API:

```bash
mimirtool rules load --address="$GRAFANA_CLOUD_MIMIR_URL" \
  --id="$GRAFANA_CLOUD_PROMETHEUS_USERNAME" --key="$GRAFANA_CLOUD_API_KEY" \
  deploy/monitoring/prometheus/rules/mgke.yml
```

а дашборд импортируется через **Dashboards** → **Import**, точно как при ручном
импорте выше.

## Alertmanager

`alertmanager/alertmanager.yml` поставляется с двумя получателями без
интеграций: алерты группируются и видны, и ничего не доставляется, пока не
подключена настоящая интеграция. Маршрут `severity = critical` эскалируется с
меньшими `group_wait` и `repeat_interval`. Получатель для Telegram подготовлен
комментарием внизу этого файла — раскомментируйте его и направьте
`bot_token_file` на смонтированный файл, чтобы получать алерты в админский чат.

## Как это держится в честности

Каждое имя `mgke_*` в этой директории проверяется дважды: preflight-проверка
`metrics` читает `deploy/monitoring/**/*.{yml,yaml,json,alloy}` и отклоняет имя,
которое скрейпящийся эндпоинт не отдаёт. Проверка подключает источники, которые
может дать конфигурация (архив, когда существует `db_path`, блок вебхука, а
также гейджи планировщика и напоминаний, пока включён `telegram.noticer`), и
прощает семейство только пока его источник действительно отсутствует, так что
настроенный источник, переставший отдавать метрики, валит проверку вместо того,
чтобы молча пройти. Затем `tests/monitoring_test.go` отклоняет имя, которого не
объявляет `api.MetricFamilies()`. Поэтому переименование в
`internal/api/metrics.go`, оставившее за собой панель дашборда, валит
`go test ./tests/...` и `go run ./scripts/preflight -skip-site`, а не
превращается в пустую панель после деплоя.

Тот же набор тестов проверяет форму JSON (версия схемы, `__inputs`,
datasources, уникальность id панелей), копии Kubernetes против обычных файлов,
стек compose (закреплённые образы, смонтированные правила и provisioning,
профиль alerts) и каждое ожидание promtool против правила, к которому оно
относится, — так что правка одного файла не может оставить остальные позади.

Одна команда прогоняет всё это — `go run ./scripts/monitoringcheck`. Она
разбирает правила и конфиг Alertmanager, сверяет копии Kubernetes, валидирует
модель импорта дашборда и поверхность имён метрик (каждое упомянутое имя
`mgke_*` должно быть и объявлено в `api.MetricFamilies()`, и реально отдаваться
ручкой, а объявленное семейство, которое больше не рендерится, тоже валит
проверку), затем запускает `promtool check rules` и `promtool test rules`. Без
promtool запуск заканчивается явным пропуском, `result: NOT VERIFIED`, и кодом
выхода 1, потому что непроверенное правило не должно читаться как успех;
передайте `-skip-promtool`, когда действительно нужно проверить только файлы.
CI-джоба `monitoring` запускает ту же команду с promtool 3.15.0, установленным
из релизного архива с закреплённой SHA-256.

# Monitoring the MGKE timetable bot

A ready-to-use monitoring set for the bot's Prometheus route (`metrics.enabled`,
`metrics.path`, `metrics.token` — see the [main README](../../README.md#prometheus-metrics)).
Nothing here needs editing before it runs: the compose stack writes the scrape
target and the bearer token itself, the dashboard is a single import, and the
rules are loadable by a plain Prometheus, the Prometheus Operator or Grafana
Cloud.

Everything is offline and reviewable: the files are configuration only (no
bundled binaries, no Grizzly, no Grafonnet, no Grafana Managed Alerts JSON), and
`tests/monitoring_test.go` fails whenever a rule, a dashboard panel or a
Kubernetes manifest mentions a metric name that `api.MetricFamilies()` does not
declare, or when the copies drift apart.

Text here is English on purpose: the repository keeps Russian user-facing text
in `internal/i18n/locales/ru.json` only, so these operator-facing assets stay
Latin.

## What is in the box

| Path | What it is |
|------|------------|
| `prometheus/prometheus.yml` | scrape config: one `mgke` job, bearer token from a mounted file, scrape target from file_sd |
| `prometheus/rules/mgke.yml` | 14 alerting rules in five groups (availability, parser, calendar, api, telegram) |
| `prometheus/rules/mgke.test.yml` | promtool unit tests for those rules |
| `alertmanager/alertmanager.yml` | Alertmanager profile with empty receivers, so alerts stay visible and nothing is delivered by accident |
| `grafana/dashboards/mgke-timetablebot.json` | the dashboard, exported in the Grafana import model (`__inputs`, `DS_PROMETHEUS`) |
| `grafana/provisioning/` | datasource and dashboard providers for the compose stack |
| `kubernetes/servicemonitor.yaml` | reference Service plus a `monitoring.coreos.com/v1` ServiceMonitor |
| `kubernetes/prometheusrule.yaml` | the same rule groups as a `monitoring.coreos.com/v1` PrometheusRule |
| `kubernetes/grafana-dashboard-configmap.yaml` | the same dashboard as a ConfigMap labelled `grafana_dashboard: "1"` for the Grafana sidecar |
| `alloy/config.alloy` | Grafana Alloy scrape and remote_write config for the Grafana Cloud case |
| `../../docker-compose.monitoring.yml` | the stack: Prometheus, Grafana and, under a profile, Alertmanager |

## Quick start with docker compose

The stack scrapes a bot that runs on the host. The only thing it needs is the
bot's metrics address, and the token when `metrics.token` is set:

```bash
export MGKE_METRICS_TARGET=host.docker.internal:8081   # api.port of the bot
export MGKE_METRICS_TOKEN='<metrics.token, leave unset when the route is open>'
export GRAFANA_ADMIN_PASSWORD='<a password you choose>'

docker compose -f docker-compose.monitoring.yml up -d
```

| Where | What |
|-------|------|
| <http://localhost:9090/targets> | the `mgke` target, `UP` once the scrape works |
| <http://localhost:9090/alerts> | the rules of `prometheus/rules/mgke.yml`, active and pending |
| <http://localhost:3000> | Grafana; the dashboard **MGKE Timetable Bot** is already provisioned in the folder *MGKE* |

With Alertmanager as well (alerts appear on <http://localhost:9093> and reach
nothing until a receiver is configured):

```bash
docker compose -f docker-compose.monitoring.yml --profile alerts up -d
```

Reload the rules after an edit without a restart:

```bash
curl -s -X POST http://127.0.0.1:9090/-/reload
```

To stop the stack and keep the data: `docker compose -f docker-compose.monitoring.yml down`.
Add `-v` only when the stored samples and the Grafana database should go too.

## Importing the dashboard by hand

Grafana also offers the dashboard as a plain file import, which asks for the
datasource at import time:

1. Grafana → **Dashboards** → **New** → **Import**.
2. Upload `deploy/monitoring/grafana/dashboards/mgke-timetablebot.json`.
3. Pick the Prometheus datasource when asked for `DS_PROMETHEUS` and import.

The exported model is `schemaVersion: 39` with an `__inputs` entry named
`DS_PROMETHEUS`; Grafana migrates it forward on import, and the dashboard keeps
working when the datasource is renamed, because every panel refers to
`${DS_PROMETHEUS}` rather than to a hard-coded uid.

## Loading the rules

Into a running Prometheus, by mounting `prometheus/rules/mgke.yml` and listing
it in `rule_files`:

```yaml
rule_files:
  - /etc/prometheus/rules/mgke.yml
```

Checking and unit-testing them takes one promtool run each:

```bash
promtool check rules deploy/monitoring/prometheus/rules/mgke.yml
promtool test rules deploy/monitoring/prometheus/rules/mgke.test.yml
```

The unit tests are the reason the parser rules look the way they do. A bot that
never parsed anything reports **no** successful parse at all, and the value of
`mgke_parser_lag_seconds` reads zero in exactly that case — "zero seconds of
lag" is not a healthy timetable, it is a parse that never happened. The rules
therefore never read the lag alone: the staleness rule combines the lag gauge
with the durable cache freshness gauge
(`mgke_cache_groups_last_update_seconds > 0 and (… freshness > 15m or lag > 15m)`),
and the never-filled case is its own rule on `mgke_uptime_seconds` plus
`mgke_cache_groups_last_update_seconds == 0`, which a lag-only rule can never
see. `mgke.test.yml` pins all three situations: a cache that was never filled, a
filled cache that went stale, and a fresh cache whose parse record went stale.

Annotations are template-free so the same strings can be asserted verbatim in
`mgke.test.yml` (promtool expands `{{ ... }}` in `exp_annotations`, so a static
string is the one form that compares equal wherever it is read).

## The alerts

| Alert | Severity | Fires when |
|-------|----------|-----------|
| [`MgkeMetricsGone`](#mgkemetricsgone) | critical | the metrics route answers nothing at all |
| [`MgkeHealthAlertsActive`](#mgkehealthalertsactive) | warning | the bot's own alert keys stay active |
| [`MgkeParserNeverFilled`](#mgkeparserneverfilled) | critical | the cache was never filled although the bot is up |
| [`MgkeParserStale`](#mgkeparserstale) | warning | neither the cache nor the parse record moved for 15 minutes |
| [`MgkeParserFrozen`](#mgkeparserfrozen) | critical | the timetable data is more than six hours old |
| [`MgkeParserFailing`](#mgkeparserfailing) | warning | two or more parse runs failed in a row |
| [`MgkeParserLayout`](#mgkeparserlayout) | warning | a required page selector stopped matching |
| [`MgkeParserGuard`](#mgkeparguard) | critical | the data-loss guard kept the previous cache |
| [`MgkeCalendarFailing`](#mgkecalendarfailing) | warning | two or more calendar syncs failed in a row |
| [`MgkeCalendarStale`](#mgkecalendarstale) | warning | no successful calendar sync for six hours |
| [`MgkeAPIErrorBurst`](#mgkeapierrorburst) | warning | three 5xx responses inside ten minutes |
| [`MgkeAPIErrorRate`](#mgkeapierrorrate) | critical | more than 2 % of at least 20 requests in 20 minutes fail |
| [`MgkeAPISlowRoute`](#mgkeapislowroute) | warning | a route averages slower than two seconds |
| [`MgkeSendFailures`](#mgkesendfailures) | warning | a Bot API transport returns errors |

### MgkeMetricsGone

`absent(mgke_uptime_seconds)` for five minutes: the bot is down, the metrics
route was switched off, or the scrape target or token is wrong. Check the
container, `metrics.enabled` and the target in `/etc/prometheus/targets`, then
`docker compose -f docker-compose.monitoring.yml up -d scrape-config`.

### MgkeHealthAlertsActive

The bot's own health layer is unhappy: the Telegram health alert already went to
`telegram.admin_ids`. The `/incidents` command lists the alert keys, the failing
API endpoints and the last 20 incidents, `/parserhealth` renders the parser
state, and `/webhook` the webhook state.

### MgkeParserNeverFilled

Up for more than 30 minutes and `mgke_cache_groups_last_update_seconds` is still
zero, so no parse ever succeeded. Parser lag reads zero here and looks healthy —
this rule is why the freshness gauge exists. Check the parser log and the live
site.

### MgkeParserStale

The cache was filled at least once, but neither the served cache nor the bot's
parse record has moved for 15 minutes. The bot serves the last good copy of the
timetable meanwhile, so this is the early warning; compare with the parse errors
and the layout probes on the dashboard.

### MgkeParserFrozen

Six hours without a successful parse: every chat is served yesterday's
timetable. Investigate the site, the parser log and the guards before the next
parse overwrites the cache entry.

### MgkeParserFailing

Two or more parse runs failed in a row. The cache keeps the previous entry, so
the timetable stays available but stale. The parser log names the failing source
and the probes that stopped matching.

### MgkeParserLayout

The last parse reported missing selectors: the college site changed its markup.
Every required probe names the CSS selector it expects, so the parser log points
straight at the block that moved.

### MgkeParserGuard

The data-loss guard tripped, which means a parse returned far less data than
before and the previous cache entry was kept. The timetable is stale rather than
empty; check the site before the next parse runs.

### MgkeCalendarFailing

Two or more calendar sync runs failed in a row. Failed days are retried because
the manual sync marker only advances after a successful pass; check the service
account, the per-chat Google account and the calendar quota.

### MgkeCalendarStale

The sync has run at least once but has not succeeded for six hours. The
reconcile pass catches missed days up as soon as one sync works, and the
`calendarSync` button on the health alert runs it by hand.

### MgkeAPIErrorBurst

Three or more 5xx responses inside ten minutes, with `keep_firing_for: 10m` so a
flapping endpoint does not resolve and re-fire on every evaluation. The bot's
`api_errors` alert names the failing endpoints; the dashboard shows which route
started answering 5xx.

### MgkeAPIErrorRate

At least 20 requests in 20 minutes and more than 2 % of them 5xx. Compare the
per-route error panel with the slowest route panel to find the handler that
degraded, and check `health.api_slow_ms`, which drives the marking inside the
bot.

### MgkeAPISlowRoute

A route averaged slower than two seconds for 15 minutes. The alert labels carry
the HTTP method and the path, so the go-to panel is "Average response time per
route".

### MgkeSendFailures

A Bot API transport returned errors in the last 20 minutes, so notifications or
replies never reached their chats. The label names the transport; blocked users
who stopped the bot and a revoked token both show up here.

## Kubernetes

```bash
kubectl apply -f deploy/monitoring/kubernetes/servicemonitor.yaml
kubectl apply -f deploy/monitoring/kubernetes/prometheusrule.yaml
kubectl apply -f deploy/monitoring/kubernetes/grafana-dashboard-configmap.yaml
```

| Manifest | How it reaches Prometheus or Grafana |
|----------|--------------------------------------|
| `servicemonitor.yaml` | a reference Service plus a `ServiceMonitor`; the kube-prometheus-stack Prometheus discovers it by the `app.kubernetes.io/name` label |
| `prometheusrule.yaml` | a `PrometheusRule` the operator loads like any other rule file |
| `grafana-dashboard-configmap.yaml` | a ConfigMap labelled `grafana_dashboard: "1"`; the Grafana sidecar copies it into Grafana, and the dashboard resolves `DS_PROMETHEUS` through its own datasource variable (default uid `prometheus`) |

Keep three things in sync with your Deployment: the pod label
`app.kubernetes.io/name: mgke-timetablebot`, a container port named `http`, and
the secret `mgke-metrics-token` with the `token` key when `metrics.token` is
set. The ServiceMonitor does not need it when the route is open — drop the
`authorization` block in that case.

The PrometheusRule is generated from the plain rules file; `tests/monitoring_test.go`
compares the two and prints the regeneration command when they drift.

## Grafana Cloud

`alloy/config.alloy` scraps the bot and pushes the samples into a Grafana Cloud
metrics instance. The credentials come from the environment, never from the
file:

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

Drop `validate` to run it. The same rules file loads into Grafana Cloud through
the rules API:

```bash
mimirtool rules load --address="$GRAFANA_CLOUD_MIMIR_URL" \
  --id="$GRAFANA_CLOUD_PROMETHEUS_USERNAME" --key="$GRAFANA_CLOUD_API_KEY" \
  deploy/monitoring/prometheus/rules/mgke.yml
```

and the dashboard imports through **Dashboards** → **Import**, exactly like the
by-hand import above.

## Alertmanager

`alertmanager/alertmanager.yml` ships two receivers without integrations: alerts
are grouped and visible, and nothing is delivered until a real integration is
plugged in. The `severity = critical` route escalates with a shorter `group_wait`
and `repeat_interval`. A Telegram receiver is prepared as a comment at the
bottom of that file — uncomment it and point `bot_token_file` at a mounted file
to get the alerts in an admin chat.

## Keeping it honest

Every `mgke_*` name in this directory is checked twice: the preflight `metrics`
check reads `deploy/monitoring/**/*.{yml,yaml,json,alloy}` and refuses a name
that the scraped endpoint does not serve (except the source-dependent families
listed in `internal/preflight/metrics.go`, which are legitimately absent when
the archive or the webhook is not wired), and `tests/monitoring_test.go` refuses
a name that `api.MetricFamilies()` does not declare. A rename in
`internal/api/metrics.go` that leaves a dashboard panel behind therefore fails
`go test ./tests/...` and `go run ./scripts/preflight -skip-site` instead of
turning into an empty panel after a deploy.

The same test suite checks the JSON shape (schema version, `__inputs`,
datasources, unique panel ids), the Kubernetes copies against the plain files,
the compose stack (pinned images, mounted rules and provisioning, the alerts
profile) and every promtool expectation against the rule it belongs to, so an
edit to one file cannot leave the others behind.

One command runs all of it — `go run ./scripts/monitoringcheck`. It parses the
rules and the Alertmanager config, checks the Kubernetes copies, validates the
dashboard import model and the metric-name surface (every referenced `mgke_*`
name must be both declared in `api.MetricFamilies()` and actually served by the
route, and a declared family that is no longer rendered fails too), then runs
`promtool check rules` and `promtool test rules`. Without promtool the run ends
with an explicit skip, `result: NOT VERIFIED`, and exit code 1, because an
unchecked rule must never read as a pass; pass `-skip-promtool` when you really
mean to check only the files. The `monitoring` CI job runs the same command with
promtool 3.15.0 installed from a SHA-256-pinned release archive.

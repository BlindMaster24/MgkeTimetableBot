# Google Calendar: setup and access rights

[Русская версия](google-calendar.md)

This document describes how to set up Google Calendar in the bot, how a user authorises, and how the reader/writer rights are managed. Go 1.27.4+ is required.

## 1. Preparing Google Cloud

1. Enable the **Google Calendar API** for the project in the Google Cloud Console.
2. Configure the **OAuth consent screen** (branding, audience, data access) and add the `.../auth/calendar` and `.../auth/userinfo.email` scopes.
3. Create an **OAuth Client ID** (type: Web application).
4. Add an **Authorized redirect URI** — it must match `google.redirect_domain + google.url` from the config **character for character**:
   - example: `https://mgke.example.com` + `/google/oauth` → `https://mgke.example.com/google/oauth`;
   - locally `http://localhost:8081/google/oauth` is allowed.
5. Create a **Service Account** and download its key: the bot needs `client_email` and `private_key`. It uses this account to create schedule calendars and to keep events in them.

## 2. Bot configuration

`configs/config.yaml` (the template is `configs/config.example.yaml`, the file itself is gitignored):

```yaml
http:
  server_name: "mgke.example.com"
  port: 8081

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

`client_id` / `client_secret` are needed for the user OAuth (access to the user's calendar list and ACL), `service_account` is needed for the calendars and events themselves. With an empty `service_account` section the sync considers itself disabled.

The public address must point at `http.port`: the same process that runs the Telegram bot handles the OAuth callback.

## 3. Linking a user account

1. In Telegram: `/google_calendar` or the **📅 Google Calendar** button in the main menu.
2. Press **Link Google account**.
3. Complete the OAuth flow and return to the chat — the bot confirms the link and offers **Configure calendar**.

After linking, the bot stores the user's email and refreshed tokens (`google_accounts` in `bot_chats.db`) and can add a schedule calendar to their account.

## 4. Adding a calendar

In the Google menu choose **Add**:

1. The bot checks that a group (student/parent mode) or a teacher (teacher mode) is selected in the chat.
2. It creates or finds the schedule calendar through the service account; the name is `Расписание занятий (<group|teacher> - <value>)`.
3. It adds the calendar to the user's calendar list and grants the reader role.

The **List** menu shows the added calendars and the sync progress: `1. group, 100 (Синхронизация: 100.00%)`. **Refresh** re-reads the list, **Delete** removes the calendar from the bot's schedule.

## 5. Access rights (reader / writer)

By default a calendar is granted with the **reader** role. To allow editing temporarily:

1. In the Google menu press **Rights**.
2. Pick the calendar.
3. Press **Grant editing rights** (writer).

To go back to reading press **Revoke editing rights** (reader).

### Roles

| Role | What it allows |
|------|----------------|
| `reader` | view only — the default mode |
| `writer` | view and edit |
| `owner` | manage the ACL — not granted to users, the owner is the service account |

## 6. Schedule synchronisation

- The data source is the SQLite schedule archive; the sync runs after every parse cycle and once at bot startup.
- Only the days that actually changed are updated: the parser reports the affected days and the bot rewrites exactly those, not the whole archive.
- A reconcile pass runs as well: when a calendar falls behind (the bot was off, a new week started), it catches up on the missing days only and never rewrites an already synced one.
- A day is processed as a whole: the old events of the day are deleted, then the new ones are created. Manual edits in the calendar are therefore not preserved.
- The sync state is stored in the `lastManualSyncedDay` column and shown as a percentage in the **List** menu.
- Lesson times come from the bell schedule: the start is the first half of the pair, the end is the second half (for example, the first pair 08:00 → 09:40); Saturday uses its own table.
- The time zone is fixed at UTC+3, with no tzdata dependency.
- Only the calendars stored in the database and linked to a chat are synced.

## 7. Common errors

### redirect_uri_mismatch

Cause: the redirect URI in Google Cloud does not match `redirect_domain + url`.

Fix: make the URI in Google Cloud and in the config identical — including the scheme, host, port and trailing slashes.

### "Google account is not linked"

Cause: the user did not complete the OAuth flow or the tokens were revoked.

Fix: press **Link Google account** in the Google menu and sign in again.

### "Failed to update access rights"

Cause: the application has no access to the calendar ACL, or the account is not authorised.

Fix: run the OAuth flow again (scope `calendar`) and check that the calendar was created by the service account and added to the user's list.

### The calendar is not syncing

Cause: the `google.service_account` section is empty, the calendar was not added through the menu, or the archive is empty.

Fix: fill in `client_email` and `private_key`, add the calendar with **Add**, and check that the bot has parsed the schedule at least once successfully.

## Links

- OAuth redirect URI and matching rules: [Google OAuth 2.0 (Web Server)](https://developers.google.com/identity/protocols/oauth2/web-server)
- OAuth consent screen setup: [Google Calendar API Auth Guide](https://developers.google.com/calendar/api/guides/auth)
- ACL roles (reader/writer/owner): [Calendar API ACL](https://developers.google.com/workspace/calendar/api/v3/reference/acl)
- Service accounts: [Service accounts overview](https://cloud.google.com/iam/docs/service-account-overview)

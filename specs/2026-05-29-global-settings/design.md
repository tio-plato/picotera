# Design: Global Settings

## Overview

A new `global_setting` table stores key-value pairs where the key is a string primary key and the value is a JSONB column. This is a general-purpose settings store that can be extended with additional keys in the future.

## Database

**Table: `global_setting`**

| Column     | Type   | Constraints      |
|------------|--------|------------------|
| `key`      | TEXT   | PRIMARY KEY      |
| `value`    | JSONB  | NOT NULL         |
| `updated_at` | TIMESTAMPTZ | NOT NULL DEFAULT now() |

This table is NOT a hypertable — it stores configuration, not time-series data. Each row represents one setting.

## API

Follows the existing pattern: contract types in `pkg/contract/`, handler methods on `*Server`, registered via `huma.Register`.

Endpoints:
- `GET /api/picotera/settings` — list all settings.
- `GET /api/picotera/settings/{key}` — get one setting by key (404 if missing).
- `PUT /api/picotera/settings` — upsert a setting (key + value).
- `DELETE /api/picotera/settings/{key}` — delete a setting by key.

The "application title" setting uses the key `app.title`. The dashboard fetches this setting on startup and reactively updates the sidebar and browser title.

## Dashboard

### Settings Page

A new `SettingsView.vue` at `/settings`. Contains a form to edit the application title. Uses the existing `Field` + `Input` + `Button` primitives.

### Application Title

- On app load, the dashboard fetches `GET /api/picotera/settings/app.title`.
- If the setting exists, its string value replaces "PicoTera" in the sidebar and becomes `document.title`.
- If missing or empty, defaults to "PicoTera".
- The title reactively updates when the setting is saved (via vue-query invalidation).
- The browser title format: `{appTitle}` (just the title, no suffix).

### Implementation

A new composable `useAppTitle` fetches the setting and provides a reactive `appTitle` ref. `AppSidebar.vue` and `App.vue` consume this ref. The composable also sets `document.title` via a watcher.

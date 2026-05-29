# Proposal: Global Settings

## Requirements

1. Add a global settings table with schema: `key` (primary key, string) - `value` (jsonb).
2. Add a global settings API (CRUD operations).
3. Add a global settings UI page in the dashboard.
4. Implement an "application title" setting:
   - When set, the sidebar title (currently "PicoTera") and the browser tab title both change to the configured value.
   - When unset or empty, defaults to "PicoTera".

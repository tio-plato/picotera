# Design: Request Status Display Change

## Overview

Modify the request status display in `RequestsView.vue` to show status state (成功/失败/处理中) instead of HTTP status codes.

## Changes

### File: `dashboard/src/views/RequestsView.vue`

1. **Simplify `requestState` function**
   - Remove dependency on `statusVariant` and `statusCode`
   - Use `r.status` field directly:
     - `status === 0 || status === 1` → `'pending'`
     - `status === 2` → `'ok'`
     - `status === 3` (or any other value) → `'err'`

2. **Update status cell template**
   - Replace status code display with text labels
   - Keep the `...` style for pending state
   - Use "成功" for ok state (green)
   - Use "失败" for err state (red)
   - Remove the `warn` state (no longer needed)

### Status Values (from API)

```go
RequestStatusPending        = 0
RequestStatusHeaderReceived = 1
RequestStatusCompleted      = 2
RequestStatusFailed         = 3
```

## UI Behavior

| Status | Display | Color |
|--------|---------|-------|
| 0 (Pending) | `...` | Gray |
| 1 (HeaderReceived) | `...` | Gray |
| 2 (Completed) | 成功 | Green (ok) |
| 3 (Failed) | 失败 | Red (err) |

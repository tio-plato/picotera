# Proposal: Request Status Display Change

## Requirements

Change the "状态" (Status) column in the requests page to display status state instead of HTTP status codes:

1. Show status state: 成功 (success) / 失败 (failure) / 处理中 (processing)
2. Processing (处理中) - keep the current style (gray `...` indicator)
3. Success (成功) - green color (same as current)
4. Failure (失败) - red color (change from yellow/warn to red/err)

## Current Implementation

The current implementation shows HTTP status codes (e.g., 200, 404, 500) with color coding:
- 2xx: green (ok)
- 4xx: yellow (warn)
- 5xx+: red (err)
- Pending: gray `...`

## Target Implementation

Use the `status` field from the API directly:
- Status 0 (Pending) or 1 (HeaderReceived) → 处理中 (processing) - gray `...`
- Status 2 (Completed) → 成功 (success) - green
- Status 3 (Failed) → 失败 (failure) - red

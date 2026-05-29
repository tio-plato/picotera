# Plan: Request Status Display Change

## Steps

1. **Update `requestState` function** in `dashboard/src/views/RequestsView.vue`
   - Change the function to use `r.status` directly instead of `r.statusCode`
   - Remove the `statusVariant` helper function (no longer needed)
   - Simplify the logic:
     - `status === 0 || status === 1` → `'pending'`
     - `status === 2` → `'ok'`
     - Otherwise → `'err'`

2. **Update status cell template** in `dashboard/src/views/RequestsView.vue`
   - Replace the `{{ row.statusCode }}` display with text labels
   - Show "成功" for ok state
   - Show "失败" for err state
   - Keep "..." for pending state (no change)

## Files to Modify

- `dashboard/src/views/RequestsView.vue` (main changes)

## Verification

1. Run `pnpm --dir dashboard type-check` to verify TypeScript compilation
2. Run `pnpm --dir dashboard lint` to check for linting issues
3. Manual verification in browser:
   - Pending requests show `...` in gray
   - Completed requests show "成功" in green
   - Failed requests show "失败" in red

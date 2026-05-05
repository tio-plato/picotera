# Plan

1. Add database migration `014_request_user_message_preview.sql` with `request.user_message_preview TEXT`, including a down migration that drops the column.

2. Update SQL queries:
   - Add `user_message_preview` to `InsertRequest`.
   - Add `user_message_preview` to `ListRequests` and `ListRequestsBySpan` select lists.
   - Add a `ListRequestTraces` lateral lookup that selects the newest meta-row preview per trace.
   - Keep existing pagination ordering unchanged.

3. Regenerate sqlc output with `sqlc generate`.

4. Implement backend extraction helper:
   - Add endpoint-type dispatch for OpenAI Chat Completions, Anthropic Messages, OpenAI Responses, and Gemini GenerateContent.
   - Add deterministic multi-format fallback for `general`, `unknown`, and non-generation endpoint types.
   - Add preview shortening by Unicode code point count.
   - Return invalid JSON, missing user messages, and unsupported text shapes as no preview.

5. Wire gateway insertion:
   - In `handle_gateway.go`, extract the preview from the inbound body and matched endpoint type before inserting the meta request row.
   - In `handle_unified_gateway.go`, extract the preview from the inbound body and source endpoint type before inserting the meta request row.
   - Ensure upstream attempt rows pass `NULL`.

6. Update contract mappings:
   - Add `UserMessagePreview` to `RequestView`, `RequestTraceView`, and internal row adapter structs.
   - Populate it in `ToRequestView`, `ToListRequestRowView`, `ToListRequestsBySpanRowView`, and `ToRequestTraceView`.

7. Add backend tests:
   - Verify each supported format extracts the last exact `role == "user"` message.
   - Verify content arrays scan from the end for the last supported text element, including Anthropic content such as `[{"type":"text","text":"foo"},{"type":"text","text":"bar"}]` producing `bar`.
   - Verify a content array whose final element is not text still uses the nearest earlier supported text element.
   - Verify a content array with no supported text element produces no preview for that message.
   - Verify preview shortening stores first 15 and last 15 code points with `...`.
   - Verify malformed JSON and unsupported shapes return no preview.
   - Verify `general`/`unknown` fallback order.

8. Regenerate API outputs:
   - Run `mise run openapi`.
   - Run `pnpm --dir dashboard generate-openapi`.

9. Update dashboard display:
   - Add `用户消息` to `TracesView.vue` near `Parent Span ID`.
   - Add `用户消息` to `RequestsView.vue` for meta/all views.
   - Add the preview to `RequestDetailsPanel.vue` overview when present.
   - Render missing values as muted `—` and apply single-line truncation with a `title`.

10. Validate:
    - Run focused Go tests for `pkg/server`.
    - Run `pnpm --dir dashboard type-check`.
    - Run `pnpm --dir dashboard build` if type-check passes.

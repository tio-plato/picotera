# Design

## Goal

Request traces will show a compact preview of the last user message seen in the inbound client request. The preview is stored on the `request` table so trace, request-list, and request-detail APIs all read the same persisted value.

## Data Model

Add nullable `request.user_message_preview TEXT`.

Only meta request rows receive this value. Upstream request rows keep it `NULL` because they can be rewritten, bridged, or retried, and the UI requirement is to show the user message from the client request received by PicoTera.

The preview format is:

- If the extracted message has 30 Unicode code points or fewer, store it unchanged.
- If it has more than 30 Unicode code points, store the first 15 code points, then `...`, then the last 15 code points.

The extractor does not trim whitespace, fold case, coerce values, or guess malformed shapes. It accepts only exact JSON structures for the known endpoint formats. Invalid JSON, missing fields, non-text content, or no user message produce `NULL`.

## Extraction

Implement a small backend helper in `pkg/server` that accepts the inbound body and endpoint type, then returns `pgtype.Text` for insertion.

Supported endpoint-type-specific extraction:

- `openaiChatCompletions`: scan `messages` from the end, find the last object with `role == "user"`, and extract text from `content`.
- `anthropicMessages`: scan `messages` from the end, find the last object with `role == "user"`, and extract text from `content`.
- `openaiResponses`: read `input`. If it is a string, use it. If it is an array, scan from the end for the last item with `role == "user"` and extract text from `content`.
- `geminiGenerateContent` and `geminiStreamGenerateContent`: scan `contents` from the end, find the last item with `role == "user"`, and extract text from `parts`.

Text extraction rules:

- String content is used directly.
- Array content is scanned from the end and uses the last supported text element.
- OpenAI/Anthropic textual parts use the last object with `type == "text"` and string `text`.
- OpenAI Responses array content uses the last object with `type == "input_text"` and string `text`.
- Gemini parts use the last object with string `text`.
- If an array has no supported text element, extraction for that message fails.

For `general`, `unknown`, and endpoint types that do not define a request-message schema, try the known generation extractors in this order: OpenAI Chat Completions, Anthropic Messages, OpenAI Responses, Gemini GenerateContent. The first successful extraction wins. This is deterministic and remains read-only; it does not make gateway routing accept invalid input.

Unified generation routes already know their source format. They pass the corresponding endpoint type to the same extractor before inserting the meta request row.

## API Shape

Expose the persisted preview in:

- `RequestView.userMessagePreview`
- `RequestTraceView.userMessagePreview`

`RequestTraceView.userMessagePreview` is selected from the newest meta request row in the trace with a non-null preview.

## Dashboard

The trace overview adds a `用户消息` column near the `Parent Span ID` column. The cell uses a single-line truncate style and sets `title` to the full stored preview. Empty values render as `—` in muted text.

The request list adds the same field for meta/all views so clicking from a trace keeps the message visible in the filtered request list. The request details overview also renders the value when present.

The implementation uses existing dashboard primitives (`AutoDataTable`, `DataCard`, and existing text/token classes) and follows `dashboard/DESIGN_SYSTEM.md`.

## Generated Artifacts

After backend contract and sqlc query changes:

- Run `sqlc generate`.
- Run `mise run openapi`.
- Run `pnpm --dir dashboard generate-openapi`.

No third-party libraries are introduced.

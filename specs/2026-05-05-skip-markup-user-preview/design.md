# Design

## Goal

Request user-message previews skip supported text blocks whose text begins with `<`. When the newest supported text block starts with `<`, extraction continues scanning earlier blocks in the same content array and uses the nearest earlier supported text block.

## Scope

This change applies to every structured content extractor in `pkg/server/user_message_preview.go`:

- OpenAI Chat Completions and Anthropic Messages content arrays handled by `extractTextContent`.
- OpenAI Responses content arrays handled by `extractInputTextContent`.
- Gemini `parts` arrays handled by `extractGeminiParts`.

The existing strict JSON handling remains unchanged:

- No whitespace trimming is added before checking for `<`.
- Only exact supported text shapes are considered.
- Unsupported content entries continue to be skipped while scanning the array.
- If every supported text block in the selected user message begins with `<`, extraction for that message fails and the preview remains absent.

This change does not modify database schema, API contracts, or dashboard rendering.

## Behavior

For all supported request formats, the extractor still scans messages from the end and selects the newest user message according to that format. Within that user message's structured content array, the extractor scans from the end. Each supported textual part is evaluated in order:

- If `text` is a string and its first byte is `<`, skip that part and continue to earlier parts.
- If `text` is a string and does not begin with `<`, return that text as the preview source.

Supported textual parts are:

- OpenAI Chat Completions and Anthropic Messages: `type == "text"` with string `text`.
- OpenAI Responses: `type == "input_text"` with string `text`.
- Gemini: any part with string `text`.

The example below returns `foobar`:

```json
{
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [{ "type": "input_text", "text": "baz" }]
    },
    {
      "type": "message",
      "role": "user",
      "content": [
        { "type": "input_text", "text": "foobar" },
        { "type": "input_text", "text": "<p></p>" }
      ]
    }
  ]
}
```

No third-party libraries are introduced.

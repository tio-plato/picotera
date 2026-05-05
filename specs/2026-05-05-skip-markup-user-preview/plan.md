# Plan

1. Update `pkg/server/user_message_preview.go`:
   - Add a small helper that returns true when a string begins with `<`.
   - Use it in `extractTextContent` for OpenAI Chat Completions and Anthropic Messages `text` parts.
   - Use it in `extractInputTextContent` for OpenAI Responses `input_text` parts.
   - Use it in `extractGeminiParts` for Gemini `text` parts.
   - Continue scanning earlier content parts when the current supported text value begins with `<`.
   - Preserve exact string handling without trimming, normalization, or coercion.

2. Add focused backend tests in `pkg/server/user_message_preview_test.go`:
   - Verify OpenAI Responses content chooses `foobar` when a newer `input_text` part is `<p></p>`.
   - Verify OpenAI Chat Completions content chooses the earlier `text` part when a newer `text` part is `<p></p>`.
   - Verify Anthropic Messages content chooses the earlier `text` part when a newer `text` part is `<p></p>`.
   - Verify Gemini parts choose the earlier `text` part when a newer `text` part is `<p></p>`.
   - Verify a text value with leading whitespace before `<` is not skipped.
   - Verify extraction returns no preview when all supported text blocks in the selected user message begin with `<`.

3. Run `go test ./pkg/server`.

4. Review the diff to confirm no schema, contract, generated OpenAPI, sqlc output, or dashboard files changed.

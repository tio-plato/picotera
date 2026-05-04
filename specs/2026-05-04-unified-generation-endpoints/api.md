# API — Unified Generation Endpoints

These three routes are gateway endpoints (handled by `gatewayHandler`-style
code), **not** Huma management operations. They are not registered under
`/api/picotera` Huma; they are mounted at the chi router root before the
catch-all gateway handler so they take priority over user-configured endpoint
rows that happen to share a prefix.

## Routes

### `POST /api/picotera/v1/messages`

- **Source format**: Anthropic Messages.
- **Auth**: any of `Authorization: Bearer …`, `x-api-key`, `?key=`,
  `x-goog-api-key`. Resolution uses the same fallback chain as the existing
  `extractClientToken` with resolver = `Unknown`.
- **Streaming**: respects `body.stream` (boolean).
- **Request body**: standard Anthropic Messages request
  (`model`, `messages`, `max_tokens`, `system`, `tools`, `tool_choice`,
  `stream`, `temperature`, `top_p`, `top_k`, `stop_sequences`, `metadata`,
  `thinking`).
- **Response**: Anthropic Messages response. SSE event names match the
  Anthropic spec (`message_start`, `content_block_start`,
  `content_block_delta`, `content_block_stop`, `message_delta`,
  `message_stop`, `ping`, `error`).

### `POST /api/picotera/v1/responses`

- **Source format**: OpenAI Responses (the new GPT-5-era endpoint).
- **Auth**: same fallback chain as above.
- **Streaming**: respects `body.stream`.
- **Request body**: OpenAI Responses request shape (`model`, `input`,
  `instructions`, `tools`, `tool_choice`, `stream`, `reasoning`, `text`).
- **Response**: OpenAI Responses response. SSE events follow OpenAI Responses
  spec (`response.created`, `response.in_progress`,
  `response.output_item.added`, `response.output_text.delta`, …,
  `response.completed`).

### `POST /api/picotera/v1/chat/completions`

- **Source format**: OpenAI Chat Completions.
- **Auth**: same fallback chain as above.
- **Streaming**: respects `body.stream`.
- **Request body**: OpenAI Chat Completions request shape
  (`model`, `messages`, `tools`, `tool_choice`, `stream`,
  `stream_options.include_usage`, `temperature`, `top_p`, `max_tokens`,
  `response_format`, `seed`, `n`).
- **Response**: OpenAI Chat Completions response. SSE chunks emit `data: {…}`
  with the standard `choices[].delta` shape and `[DONE]` terminator.

### `POST /api/picotera/v1beta/models/{model}:generateContent`

- **Source format**: Gemini GenerateContent (non-streaming).
- **Model**: read from the chi path variable `{model}` — the request body's
  body has no model field.
- **Auth**: same fallback chain (`?key=` and `x-goog-api-key` are the
  idiomatic ones for Gemini SDKs and resolve naturally).
- **Streaming**: always false. Even if the upstream we route to is a
  streaming endpoint, axonhub's transformer aggregates chunks into a
  non-streaming Gemini response before we hand it to the client.
- **Request body**: standard Gemini GenerateContent request
  (`contents`, `systemInstruction`, `tools`, `toolConfig`,
  `generationConfig`, `safetySettings`).
- **Response**: Gemini GenerateContent response (`candidates`, `usageMetadata`,
  `promptFeedback`).

### `POST /api/picotera/v1beta/models/{model}:streamGenerateContent`

- **Source format**: Gemini GenerateContent (streaming).
- **Model**: read from the chi path variable `{model}`.
- **Auth**: same fallback chain.
- **Streaming**: always true. Emits Gemini's SSE flavour
  (`data: {…}\n\n` with each chunk being a `GenerateContentResponse`
  shape).

## MPE selection

For each route the handler computes the candidate endpoint-type set:

| Route                                                       | `stream:false` set                                                                       | `stream:true` set                                                                            |
| ----------------------------------------------------------- | ---------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `/v1/messages`                                              | anthropicMessages, openaiChatCompletions, openaiResponses, geminiGenerateContent         | anthropicMessages, openaiChatCompletions, openaiResponses, geminiStreamGenerateContent       |
| `/v1/responses`                                             | anthropicMessages, openaiChatCompletions, openaiResponses, geminiGenerateContent         | anthropicMessages, openaiChatCompletions, openaiResponses, geminiStreamGenerateContent       |
| `/v1/chat/completions`                                      | anthropicMessages, openaiChatCompletions, openaiResponses, geminiGenerateContent         | anthropicMessages, openaiChatCompletions, openaiResponses, geminiStreamGenerateContent       |
| `/v1beta/models/{model}:generateContent`                    | anthropicMessages, openaiChatCompletions, openaiResponses, geminiGenerateContent         | — (route is non-streaming)                                                                   |
| `/v1beta/models/{model}:streamGenerateContent`              | — (route is streaming)                                                                    | anthropicMessages, openaiChatCompletions, openaiResponses, geminiStreamGenerateContent       |

Sort order is unchanged: candidates sorted by combined priority
(`provider.priority + provider_models[].priority`), highest first, then handed
to the JS `sortProviders` hook for an optional reorder.

## Errors

Same envelope as the existing gateway: `{message, code, details: []}`.

| Condition                                         | Status | `code`                |
| ------------------------------------------------- | ------ | --------------------- |
| Source body fails `Inbound.TransformRequest`      | 400    | `MODEL_NOT_FOUND` if missing model, otherwise `INVALID_REQUEST` (new errorx code) |
| No upstream supports the model+stream combination | 404    | `NO_PROVIDER_AVAILABLE` |
| Bridge conversion fails on a chosen candidate     | 502    | `UPSTREAM_ERROR` (with bridge-specific message) |
| All candidates failed                              | 502    | `UPSTREAM_ERROR`      |

`INVALID_REQUEST` is added to `pkg/errorx` if not already present.

## Hook contract changes

The five existing hooks keep their signatures and visible shapes. The only
new fact a script may want to observe: when a source-vs-upstream format
mismatch is about to occur, the candidate's `mpe.endpointPath` and the
`endpoint.endpointType` differ from `request.path`'s implied format. We do
**not** introduce a new hook for the bridge. Scripts that want to gate on the
mismatch can read `mpe.endpointPath` and decide based on it.

## OpenAPI

These routes are **not** advertised in `openapi.yaml`. The Huma spec keeps
covering only the management API; the gateway routes are documented in this
file and in code comments.

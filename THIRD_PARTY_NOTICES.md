# Third-party notices

PicoTera links against the following third-party Go modules whose licenses
require attribution.

## llmbridge.wasm / github.com/looplj/axonhub/llm

PicoTera can load an optional `llmbridge.wasm` component to convert LLM
request and response payloads between Anthropic Messages, OpenAI Chat
Completions, OpenAI Responses, and Gemini GenerateContent formats. The
component is built from `cmd/llmbridge-wasm/` and `pkg/llmbridgeimpl/`.
The main `picotera` binary does not embed this module; operators enable it
by setting `PICOTERA_LLMBRIDGE_WASM_PATH` to an external WASM file.

The `llmbridge.wasm` component imports the `llm/` sub-tree of
<https://github.com/looplj/axonhub> (the `transformer`, `streams`,
`httpclient`, and core packages).

That sub-tree is distributed under the **GNU Lesser General Public License,
Version 3 (LGPL-3.0)**. The full license text is reproduced in the upstream
repository at:

  <https://github.com/looplj/axonhub/blob/main/llm/LICENSE>

The pinned module version is:

  `github.com/looplj/axonhub/llm v0.0.0-20260504030509-3a5f34936974`

Rebuild the component after changing `pkg/llmbridgeimpl/`,
`cmd/llmbridge-wasm/`, or the pinned axonhub module:

  `GOOS=wasip1 GOARCH=wasm go build -trimpath -ldflags=-buildid= -buildmode=c-shared -o dist/llmbridge.wasm ./cmd/llmbridge-wasm`

The default Docker runtime target contains only `/app/picotera`. The
`runtime-lgpl` target reuses that same binary layer, adds
`/app/llmbridge.wasm`, and sets
`PICOTERA_LLMBRIDGE_WASM_PATH=/app/llmbridge.wasm`. Operators can mount a
replacement module into either image and point `PICOTERA_LLMBRIDGE_WASM_PATH`
at that exact file.

No source modifications to the axonhub `llm/` sub-tree have been made;
picotera only imports it in the separately built WASM component.

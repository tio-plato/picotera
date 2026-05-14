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

The `llmbridge.wasm` component imports a local copy of the `llm/` sub-tree of
<https://github.com/looplj/axonhub> (the `transformer`, `streams`,
`httpclient`, and core packages).

That sub-tree is distributed under the **GNU Lesser General Public License,
Version 3 (LGPL-3.0)**. The full license text is reproduced in the upstream
repository at:

  <https://github.com/looplj/axonhub/blob/main/llm/LICENSE>

The pinned module version is:

  `github.com/looplj/axonhub/llm v0.0.0-20260504030509-3a5f34936974`

PicoTera carries that sub-tree under `third_party/axonhub/llm` so the TinyGo
WASI build can exclude network-only helpers that are not used by the
conversion module. The TinyGo-specific replacements fail if invoked.

Rebuild the component after changing `pkg/llmbridgeimpl/`,
`cmd/llmbridge-wasm/`, or the pinned axonhub module copy:

  `mise run wasm`

The default Docker runtime target contains only `/app/picotera`. The
`runtime-lgpl` target reuses that same binary layer, adds
`/app/llmbridge.wasm`, and sets
`PICOTERA_LLMBRIDGE_WASM_PATH=/app/llmbridge.wasm`. Operators can mount a
replacement module into either image and point `PICOTERA_LLMBRIDGE_WASM_PATH`
at that exact file.

The main `picotera` binary only imports the host-side WASM client. AxonHub
transformer code is used by the separately built WASM component.

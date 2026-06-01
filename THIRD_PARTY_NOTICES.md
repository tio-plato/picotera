# Third-party notices

PicoTera links against the following third-party Go modules whose licenses
require attribution.

## picotera-llmbridge-plugin / github.com/looplj/axonhub/llm

PicoTera can load an optional `picotera-llmbridge-plugin` component to convert LLM
request and response payloads between Anthropic Messages, OpenAI Chat
Completions, OpenAI Responses, and Gemini GenerateContent formats. The
component is built from `cmd/picotera-llmbridge-plugin/` and
`pkg/llmbridgeimpl/`. The main `picotera` binary does not embed this
component; operators enable it by setting `PICOTERA_LLMBRIDGE_PLUGIN_PATH`
to an external executable path.

The `picotera-llmbridge-plugin` component imports a local copy of the `llm/`
sub-tree of
<https://github.com/looplj/axonhub> (the `transformer`, `streams`,
`httpclient`, and core packages).

That sub-tree is distributed under the **GNU Lesser General Public License,
Version 3 (LGPL-3.0)**. The full license text is reproduced in the upstream
repository at:

  <https://github.com/looplj/axonhub/blob/main/llm/LICENSE>

The pinned module version is:

  `github.com/looplj/axonhub/llm v0.0.0-20260504030509-3a5f34936974`

PicoTera carries that sub-tree under `third_party/axonhub/llm`.

Rebuild the component after changing `pkg/llmbridgeimpl/`,
`cmd/picotera-llmbridge-plugin/`, or the pinned axonhub module copy:

  `mise run llmbridge-plugin`

The Docker runtime image contains `/app/picotera` and
`/app/picotera-llmbridge-plugin`, and sets
`PICOTERA_LLMBRIDGE_PLUGIN_PATH=/app/picotera-llmbridge-plugin`. Operators can
mount a replacement executable into the image and point
`PICOTERA_LLMBRIDGE_PLUGIN_PATH` at that exact file.

The main `picotera` binary only imports the host-side plugin client. AxonHub
transformer code is used by the separately built plugin component.

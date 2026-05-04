# Third-party notices

PicoTera links against the following third-party Go modules whose licenses
require attribution.

## github.com/looplj/axonhub/llm

PicoTera imports the `llm/` sub-tree of <https://github.com/looplj/axonhub>
(the `transformer`, `streams`, `httpclient`, and core packages) to convert
LLM request and response payloads between Anthropic Messages, OpenAI Chat
Completions, OpenAI Responses, and Gemini GenerateContent formats. The
adapter lives in `pkg/llmbridge/`.

That sub-tree is distributed under the **GNU Lesser General Public License,
Version 3 (LGPL-3.0)**. The full license text is reproduced in the upstream
repository at:

  <https://github.com/looplj/axonhub/blob/main/llm/LICENSE>

Per LGPL-3.0 §4, picotera links against the package as a Go module
dependency. Downstream consumers are free to substitute a modified version
of the library by setting a `replace` directive in `go.mod` against
`github.com/looplj/axonhub/llm`. The pinned upstream version is recorded in
`go.sum`.

No source modifications to the axonhub `llm/` sub-tree have been made;
picotera only imports it.

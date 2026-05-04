## Proposal

jsx.Candidate 的 MPE 和 Providers 应该改为 struct 类型，不要用 any。

> Scope confirmed: Candidate.{Provider, MPE} + BeforeRequestInput.{Provider, MPE} + RewriteInput.{Provider, MPE}. Annotations stays `json.RawMessage`.

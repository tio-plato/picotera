# Proposal — Parent Span Traces

控制台增加按 parent_span_id 列出请求的功能“追踪”，点击之后列表页显示所有已知的 parent_span_id 以及对应的请求数、tokens 总数、总成本（当然，也有分页）。点击其中一个，会打开一个新页面，这个新页面复用“请求”界面，但里面只列出所有指定 parent_span_id 的请求，类似做了筛选吧，我觉得跳到请求页面加个 url 参数去做筛选就行了。

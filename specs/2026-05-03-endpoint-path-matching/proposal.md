# Endpoint Path Matching with Variables

给 endpoint 增加匹配功能，具体来说，就是支持形如 `/v1beta/models/{model}:generateContent` 这样子的路由，其中 `{model}` 解析为通配符，可以匹配任何内容（包括斜线）。在处理这类请求的时候，将路径变量解析为 `map[string]string` ，在提取模型 path 时，除了匹配 body 之外，还应该匹配路径变量（优先路径变量）。在处理上游 URL 的时候，也需要能够替换上游 URL 里的路径变量。

因为这会使得 endpoint 的匹配过程变得动态，所以我们在匹配的时候，应该将所有 endpoint 都读取到内存里，在内存里运行匹配。为了避免经常读取，这部分需要做个简单的内存缓存，在重启或对路由进行编辑时自动失效，需要时重新读取即可。

由于引入了一个缓存，我们还需要更新 CLAUDE.md 和进行恰当的注释，使得之后的 agent 在看到 endpoint 匹配规则的时候能够知道这里有缓存。

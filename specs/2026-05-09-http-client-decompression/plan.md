# 执行计划

1. 增加响应解压辅助模块
   - 新建 `pkg/server/response_decompression.go`。
   - 实现 `decodedBody(resp *http.Response) (*decodedResponseBody, error)`。
   - 实现 `decodedReadCloser(src io.ReadCloser, encoding string) (io.ReadCloser, error)`。
   - 支持 `gzip`、`br`、`zstd` 和空 encoding。
   - 对未知 encoding、多层 encoding、空白变体直接返回错误。
   - 给 zstd reader 增加释放 decoder 的 `Close` 包装。

2. 补齐解压单元测试
   - 新建 `pkg/server/response_decompression_test.go`。
   - 覆盖 gzip、br、zstd、无 encoding、未知 encoding、组合 encoding。
   - 测试 `Close` 会关闭底层 reader。

3. 改造 path gateway 成功响应流
   - 修改 `streamSuccess`。
   - 保留现有响应头复制规则：跳过 `Content-Length`，保留 `Content-Encoding`。
   - 对带 `Content-Encoding` 的响应建立原始 tee：
     - 主路径写客户端。
     - 副路径进入 pipe 后解压。
   - `ResponseExtractor` 从解压 reader 读取。
   - capture buffer 写解压 bytes。
   - `uploadResponseArtifactWithAggregation` 和 `uploadMetaResponseArtifactWithAggregation` 使用解压 bytes。
   - `buildAggregatedArtifact` 使用解压 bytes 和原始 `Content-Type`。
   - 无 `Content-Encoding` 时保持当前单路读取行为。

4. 改造 path gateway 非 200 分支
   - 在 `handle_gateway.go` 非 200 分支使用 `decodedBody(resp)`。
   - artifact body、`errMsg`、`LastError.Message` 使用解压后的 body。
   - 解压失败时关闭原始 body，记录失败 attempt，错误消息为 `decode upstream response: ...`。

5. 改造 unified gateway 非 200 分支
   - 在 `handle_unified_gateway.go` 非 200 分支使用 `decodedBody(resp)`。
   - artifact body、`errMsg`、`LastError.Message` 使用解压后的 body。
   - 解压失败时按 upstream attempt failed 处理。

6. 改造 unified gateway 非桥接成功路径
   - 修改 `unifiedStreamSuccess` 中 `bridging == false` 分支。
   - 带 `Content-Encoding` 时用双路读取：
     - 原始 bytes 写客户端。
     - 解压 bytes 进入 extractor、upstream capture、client/meta capture。
   - meta artifact body 使用解压 bytes。
   - upstream artifact body 使用解压 bytes。
   - aggregation 使用解压 bytes。
   - 客户端 header 保留上游 `Content-Encoding`。

7. 改造 unified gateway 桥接成功路径
   - 修改 `unifiedStreamSuccess` 中 `bridging == true` 分支。
   - `decodedBody(resp)` 作为 extractor 输入。
   - stream bridge 和 non-stream bridge 消费解压后的 upstream bytes。
   - upstream capture 保存解压后的 upstream bytes。
   - 转发 header 时额外跳过上游 `Content-Encoding`。
   - 保持桥接输出不设置压缩编码。

8. 改造 fetch models
   - 修改 `handleFetchModels`。
   - 非 2xx 错误摘要读取解压 body。
   - 成功 body 读取解压 body 后再传给 `parseModelsResponse`、`json.Unmarshal` 和 `rewriteProviderModels` hook。
   - 解压失败返回 502。

9. 加网关集成测试
   - 为 path gateway 增加压缩响应测试。
   - 使用 gzip SSE 或 JSON fixture 验证客户端收到压缩体，内部 metrics/artifact 使用解压体。
   - 为 unified bridge 增加压缩 non-stream upstream 测试，验证 llmbridge 消费解压后的 JSON。
   - 为非 200 分支增加压缩错误体测试，验证错误消息为解压文本。

10. 验证
    - 运行 `go test ./pkg/server ./pkg/llmbridge`。
    - 运行 `go test ./...`。
    - 手动检查 `response_decompression.go` 没有宽松 normalization、兼容 fallback 或静默忽略未知 encoding。
    - 手动检查成功响应给客户端的 header/body 仍保持上游压缩语义。

# API — JSX Console Logs in Artifacts

## Huma 管理 API

无新增、无变更。Logs 不通过 management API 暴露，只随 meta response artifact JSON 一起下发。

## Artifact JSON Shape

`<id>.response.json.zst` 的 JSON 在 meta 请求上新增可选字段 `logs`。upstream 的 response artifact 不变。

```json
{
  "statusCode": 200,
  "headers": { "Content-Type": ["application/json"] },
  "body": "...",
  "bodyEncoding": "utf8",
  "logs": [
    {
      "level": "info",
      "message": "rewriteRequest: applying claude transform",
      "ts": "2026-05-01T08:32:11.123456789Z"
    },
    {
      "level": "warn",
      "message": "fallback to provider id=3",
      "ts": "2026-05-01T08:32:11.512345678Z"
    },
    {
      "level": "error",
      "message": "upstream returned 503",
      "ts": "2026-05-01T08:32:11.987654321Z"
    }
  ]
}
```

字段约束：

| 字段       | 类型     | 说明                                                         |
| ---------- | -------- | ------------------------------------------------------------ |
| `level`    | string   | `"info" \| "warn" \| "error"`，SDK 的 `debug` 已映射为 `info` |
| `message`  | string   | 多个参数已在 JS 侧 `parts.join(' ')`；单条上限 8KB，超长截尾 `... [truncated]` |
| `ts`       | string   | RFC3339Nano，UTC 时区                                        |

数组顺序为追加顺序（即 console 调用顺序）。

裁剪哨兵（仅在触达上限时附在末尾）：

```json
{ "level": "warn", "message": "[picotera] log buffer truncated", "ts": "..." }
```

兼容性：旧 artifact 不带 `logs` 字段；前端把缺省视作空数组。

## 限额（硬编码）

| 名称              | 值          |
| ----------------- | ----------- |
| 单条 message      | 8 KB        |
| 累计 message 字节 | 256 KB      |
| 条数              | 1000        |

## Console SDK（前端 / 脚本侧）

无变更，仍是 `pkg/jsx/sdk.js` 暴露的 `console.{log,info,warn,error,debug}`。

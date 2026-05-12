# API: JS KV Access (`picotera.kv.*`)

## JS API

所有方法均为 async，返回 Promise。

### `picotera.kv.get(key: string): Promise<string | null>`

获取 key 对应的值。key 不存在或已过期时返回 `null`。

```js
const val = await picotera.kv.get("my-key");
if (val === null) {
  console.log("key not found");
} else {
  console.log("value:", val);
}
```

### `picotera.kv.set(key: string, value: string): Promise<void>`

设置 key-value，无过期时间。key 已存在则覆盖。

```js
await picotera.kv.set("my-key", "my-value");
```

### `picotera.kv.setex(key: string, seconds: number, value: string): Promise<void>`

设置 key-value 并附带 TTL（秒）。key 已存在则覆盖。

```js
await picotera.kv.setex("temp-key", 60, "expires-in-60s");
```

### `picotera.kv.ttl(key: string): Promise<number>`

查询 key 的剩余生存时间（秒）。

| 返回值 | 含义 |
|--------|------|
| `-2`   | key 不存在或已过期 |
| `-1`   | key 存在但无过期时间 |
| `>= 0` | 剩余秒数（向上取整） |

```js
const remaining = await picotera.kv.ttl("temp-key");
console.log("expires in", remaining, "seconds");
```

### `picotera.kv.del(key: string): Promise<void>`

删除 key。key 不存在时不报错。

```js
await picotera.kv.del("my-key");
```

## Config Environment Variables

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PICOTERA_KV_DRIVER` | `memory` | 驱动选择：`memory` 或 `redis` |
| `PICOTERA_KV_REDIS_URL` | `localhost:6379` | Redis 连接地址 |

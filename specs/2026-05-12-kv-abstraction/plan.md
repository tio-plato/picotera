# Plan: KV Abstraction Package

## Step 1: Add Dependencies

```bash
go get github.com/jellydator/ttlcache/v3@latest
go get github.com/redis/go-redis/v9@latest
```

## Step 2: Create `pkg/kv/store.go`

定义核心接口和哨兵错误：

- `Store` 接口：`Get`, `Set`, `SetEx`, `TTL`, `Del`, `Close`
- `var ErrKeyNotFound = errors.New("kv: key not found")`

## Step 3: Create `pkg/kv/memory.go`

实现 ttlcache 后端：

- `MemoryStore` 结构体，持有 `*ttlcache.Cache[string, string]`
- `NewMemoryStore()` 构造函数：
  - `ttlcache.New[string, string]()` 创建 cache 实例
  - `go cache.Start()` 启动内置过期清理
- `Get`: `cache.Get(key)` → nil 返回 `ErrKeyNotFound`，否则返回 `item.Value()`
- `Set`: `cache.Set(key, value, ttlcache.NoTTL)`
- `SetEx`: `cache.Set(key, value, ttl)`
- `TTL`: `cache.Get(key)` → nil 返回 -2 → 检查 item 是否有 TTL（`HasExpiration`）→ 无 TTL 返回 -1 → 有 TTL 返回 `math.Ceil(item.RemainingTTL().Seconds())`
- `Del`: `cache.Delete(key)`，key 不存在不报错
- `Close`: `cache.Stop()`

## Step 4: Create `pkg/kv/redis.go`

实现 redis 后端：

- `RedisStore` 结构体，持有 `*redis.Client`
- `NewRedisStore(url string)` 构造函数：
  - 解析 `host:port` 格式 URL（不含 scheme）
  - 创建 `redis.Client{Addr: url}`
  - `Ping` 验证连接
- `Get`: `client.Get(ctx, key).Result()` → `redis.Nil` 转为 `ErrKeyNotFound`
- `Set`: `client.Set(ctx, key, value, 0)`
- `SetEx`: `client.Set(ctx, key, value, ttl)`
- `TTL`: `client.TTL(ctx, key).Result()` → key 不存在返回 -2 → `-2 * time.Second`（redis 对不存在 key 返回 -2ns）返回 -1 → 秒数
- `Del`: `client.Del(ctx, key)`
- `Close`: `client.Close()`

## Step 5: Create `pkg/kv/kv.go`

工厂函数：

```go
func New(driver string, opts ...Option) (Store, error)
```

- functional options：`WithRedisURL(url)`
- `driver == "memory"` → `NewMemoryStore()`（无额外参数）
- `driver == "redis"` → `NewRedisStore(url)`，默认 `localhost:6379`
- 其他值返回错误

## Step 6: Add Config to `pkg/configx/configx.go`

在 `Config` 结构体中新增：

```go
KV KVConfig `mapstructure:"kv"`
```

新增 `KVConfig` 结构体：

```go
type KVConfig struct {
    Driver   string `mapstructure:"driver"`
    RedisURL string `mapstructure:"redis_url"`
}
```

在 `Parse()` 中添加默认值：

```go
viper.SetDefault("kv.driver", "memory")
viper.SetDefault("kv.redis_url", "localhost:6379")
```

## Step 7: Wire KV Store into JSX Engine

修改 `pkg/jsx/engine.go`：

- `Engine` 新增 `kvStore kv.Store` 字段
- `NewEngine` 签名新增必选参数 `kvStore kv.Store`

修改 `pkg/jsx/helpers.go`：

- `registerHelpers` 中调用新的 `registerKV(s)` 函数

`registerKV(s *Session)` 注册五个 async 函数：

| Go 函数 | 实现 |
|---------|------|
| `__picotera_kv_get(key)` | `kvStore.Get(ctx, key)` → resolve 值字符串或 null |
| `__picotera_kv_set(key, value)` | `kvStore.Set(ctx, key, value)` → resolve undefined |
| `__picotera_kv_setex(key, seconds, value)` | `kvStore.SetEx(ctx, key, value, time.Duration(seconds)*time.Second)` → resolve undefined |
| `__picotera_kv_ttl(key)` | `kvStore.TTL(ctx, key)` → resolve 数字 |
| `__picotera_kv_del(key)` | `kvStore.Del(ctx, key)` → resolve undefined |

所有错误通过 `Promise().Reject(err)` 返回。

## Step 8: Update `sdk.js`

在 `globalThis.picotera` 对象上新增 `kv` 属性：

```js
kv: {
  get: function(key) { return globalThis.__picotera_kv_get(String(key)).then(function(s) { return s === '' ? null : s; }) },
  set: function(key, value) { return globalThis.__picotera_kv_set(String(key), String(value)) },
  setex: function(key, seconds, value) { return globalThis.__picotera_kv_setex(String(key), Number(seconds), String(value)) },
  ttl: function(key) { return globalThis.__picotera_kv_ttl(String(key)) },
  del: function(key) { return globalThis.__picotera_kv_del(String(key)) },
}
```

## Step 9: Update `server.go`

在 `NewServer` 中：

1. 创建 KV store：`kvStore, err := kv.New(config.KV.Driver, kv.WithRedisURL(config.KV.RedisURL))`
2. 传入 `jsx.NewEngine`：`jsxEngine := jsx.NewEngine(jsxCfg, queries, kvStore)`

## Step 10: Verify

1. `go build ./...` 确认编译通过
2. `go vet ./...` 检查代码质量

# Design: KV Abstraction Package

## Overview

新增 `pkg/kv/` 包，提供统一的字符串 KV 存储接口，支持 memory（默认，基于 ttlcache）和 redis 两种后端驱动。KV store 作为全局单例注入 JSX 引擎，JS 脚本通过 `picotera.kv.*` 命名空间访问。

## Architecture

### Package Layout

```
pkg/kv/
  store.go    — Store 接口定义 + ErrKeyNotFound 哨兵错误
  memory.go   — ttlcache 实现（原生 TTL，内置过期清理）
  redis.go    — redis 实现，利用原生命令
  kv.go       — New() 工厂函数，根据 driver 字符串选择实现
```

### Store Interface

```go
type Store interface {
    Get(ctx context.Context, key string) (string, error)       // key 不存在返回 ("", ErrKeyNotFound)
    Set(ctx context.Context, key, value string) error           // 无过期
    SetEx(ctx context.Context, key, value string, ttl time.Duration) error
    TTL(ctx context.Context, key string) (int64, error)         // -2=不存在, -1=无过期, >=0=剩余秒数
    Del(ctx context.Context, key string) error
    Close() error
}
```

### Memory Driver (ttlcache)

- 基于 `github.com/jellydator/ttlcache/v3`，泛型 `Cache[string, string]`。
- 原生支持 per-item TTL，无需手动管理过期时间戳。
- `cache.Start()` 启动内置的过期清理 goroutine，`cache.Stop()` 停止。
- Set 使用 `ttlcache.NoTTL`，SetEx 使用具体 TTL duration。
- Get 返回 nil 时映射为 `ErrKeyNotFound`。
- TTL 通过 `item.RemainingTTL()` 获取剩余时间；item 存在但无 TTL 返回 -1。

### Redis Driver

- 使用 `github.com/redis/go-redis/v9`。
- TTL 操作直接映射到 Redis 原生命令（GET/SET/SETEX/TTL/DEL）。
- 无需额外过期管理逻辑。

### Config

在 `configx.Config` 中新增：

```go
KV KVConfig `mapstructure:"kv"`

type KVConfig struct {
    Driver   string `mapstructure:"driver"`    // "memory" | "redis"，默认 "memory"
    RedisURL string `mapstructure:"redis_url"` // 默认 "localhost:6379"
}
```

环境变量: `PICOTERA_KV_DRIVER`, `PICOTERA_KV_REDIS_URL`

### JSX Integration

**Engine 改动**: `Engine` 新增 `kvStore kv.Store` 字段，`NewEngine` 签名新增必选参数 `kvStore kv.Store`。

**Session 改动**: Session 通过 `s.engine.kvStore` 访问 KV store。`registerHelpers` 中调用新的 `registerKV(s)` 注册五个 Go 侧异步函数。

**Go 侧函数** (async, 通过 `__picotera_kv_*` 暴露):

| Go 函数 | JS 调用 | 行为 |
|---------|---------|------|
| `__picotera_kv_get(key)` | `picotera.kv.get(key)` | 返回值字符串或 null |
| `__picotera_kv_set(key, value)` | `picotera.kv.set(key, value)` | 无返回 |
| `__picotera_kv_setex(key, seconds, value)` | `picotera.kv.setex(key, seconds, value)` | 无返回 |
| `__picotera_kv_ttl(key)` | `picotera.kv.ttl(key)` | 返回数字 |
| `__picotera_kv_del(key)` | `picotera.kv.del(key)` | 无返回 |

所有函数均为 Promise-based（`SetAsyncFunc`），与 `picotera.fetch` 风格一致。

**sdk.js 改动**: 在 `globalThis.picotera` 上新增 `kv` 对象，封装 `__picotera_kv_*` 为 Promise API。

### Dependencies

- `github.com/jellydator/ttlcache/v3` — 内存 KV store，原生 TTL
- `github.com/redis/go-redis/v9` — Redis client

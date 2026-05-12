# Proposal: KV Abstraction Package

增加一个 kv 抽象包，支持配置由 ttlcache（内存）驱动或者由 redis 驱动，默认是 memory（基于 github.com/jellydator/ttlcache/v3，纯内存，原生 TTL 支持）。提供基本的字符串类型的 kv 操作函数，如 get, set, setex, ttl, del 等，暴露给 js 环境，使得 js 运行时可以自由访问 kv 存储。

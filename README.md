# PicoTera

一款偏好明确的个人 LLM 网关。vibe coding 产物，包含 99% 以上的 AI 生成代码。

项目尚在开发过程中，但主要功能已可用。现阶段我们没有 Docker image 或是预编译二进制提供。

## 功能特点

* 通过脚本定义各类路由行为
* 默认透传；可集成 AxonHub 转换库并自由配置转换参数，供不时之需
* 基于工作目录自动识别项目，并分别统计成本
* 多币种费用统计
* 完整的请求/响应日志

## 安装

### 外部依赖

* TimescaleDB - 必选
* Redis - 可选：用于脚本 KV 存储，如果没有则自动回落自带内存 KV 引擎
* S3 兼容的对象存储 - 可选：用于请求/响应存储，如果没有则不会记录请求响应头/体，只有部分元数据被记录

### 配置样例

```
PICOTERA_DATABASE_URL=postgres://picotera:picotera@localhost:34052/picotera
PICOTERA_S3_ENDPOINT=localhost:34050
PICOTERA_S3_REGION=us-east-1
PICOTERA_S3_ACCESS_KEY=picotera
PICOTERA_S3_SECRET_KEY=picotera-dev
PICOTERA_S3_USE_SSL=false
PICOTERA_S3_BUCKET=picotera-artifacts
PICOTERA_S3_FORCE_PATH_STYLE=true
PICOTERA_S3_PUBLIC_URL=http://localhost:34050
```

### 请求转换组件

请求转换组件使用 AxonHub 的 LGPL 代码，因而需要单独编译，通过 wasm 模块链接使用。

### 优化 Timescaledb 参数

```bash
docker compose exec -it postgres timescaledb-tune --yes -cpus 1 -memory 512MB
```

## 协议

* `cmd/llmbridge-wasm`: LGPLv3
* `pkg/llmbridgeimpl`: LGPLv3
* 其它： BSD 3-Clause


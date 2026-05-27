# PicoTera

一款偏好明确的个人 LLM 网关。vibe coding 产物，包含 99% 以上的 AI 生成代码。

项目尚在开发过程中，但主要功能已可用。

## 功能特点

* 通过脚本定义各类路由行为
* 默认透传；可集成 AxonHub 转换库并自由配置转换参数，供不时之需
* 基于工作目录自动识别项目，并分别统计成本
* 多币种费用统计
* 完整的请求/响应日志

## 界面展示

### 概览页

流量结构桑基图

![](./docs/screenshots/overview1.webp)

各类堆叠图、饼图

![](./docs/screenshots/overview2.webp)

### 端点页

自由设置访问端点

![](./docs/screenshots/endpoints.webp)

### 脚本页

自定义行为

![](./docs/screenshots/scripts.webp)

### 请求详情页

可看当前元请求（客户端请求）和上游请求（服务端请求）的完整详细信息

![](./docs/screenshots/requests.webp)

## 安装

> [!CAUTION]
> 本项目的管理 API 目前没有鉴权，因此请勿将其暴露到公网使用！

### Docker

请使用如下镜像运行 picotera ，并发布 9898 端口：

```
ghcr.io/oott123/picotera:master
```

该镜像包含 LGPL 的 wasm 组件，位于 `/app/llmbridge.wasm` 。

根据 LGPL 许可证的授权，你可以自由地通过挂载文件以替换该组件。

### Docker Compose

请参考 [docker-compose.yaml](./docs/deploy/docker-compose.yaml)。

特别注意，请修改 minio 的默认密码，防止 minio 被未授权访问。

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
默认构建使用 TinyGo 生成 size 优化的 WASI reactor，同时保留 DWARF/name 调试信息：

```bash
mise run wasm
```

生成的 `dist/llmbridge.wasm` 需要通过 `PICOTERA_LLMBRIDGE_WASM_PATH` 提供给主程序。

如果遇到 `llmbridge: allocate guest memory`、`wasm error: out of bounds memory access` 之类的错误，需要排查 wasm 内部堆栈时，可以切到 interpreter runtime 复现，堆栈通常更稳定：

```bash
PICOTERA_LLMBRIDGE_WASM_RUNTIME=interpreter mise run server
```

宿主侧会把 wasm stdout/stderr 一并附在 `llmbridge` 错误里。

### 优化 Timescaledb 参数

```bash
docker compose exec -it postgres timescaledb-tune --yes -cpus 1 -memory 512MB
```

## 协议

* `cmd/llmbridge-wasm`, `pkg/llmbridgeimpl`, `third_party/axonhub`: LGPLv3
* 其它： BSD 3-Clause

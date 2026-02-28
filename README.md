# iflow-go

`iflow-go` 是 `iflow2api` 的 Go 语言实现，提供 OpenAI 兼容代理接口。

## 功能

- OpenAI 兼容端点：`/v1/chat/completions`
- 多账号管理：使用 `Bearer <uuid>` 路由到对应账号
- OAuth 登录与 Token 刷新
- CLI 命令管理（无 Web 后台）

## 环境要求

- Go 1.25+

## 安装与构建

```bash
git clone <repo-url>
cd iflow-go
go mod tidy
make build
```

生成的可执行文件位于 `bin/iflow-go`。

## 快速开始

1. 复制环境变量模板：

```bash
cp .env.example .env
```

2. 导入账号（OAuth）：

```bash
go run . token import
```

3. 启动服务：

```bash
go run . serve --host 0.0.0.0 --port 28000
```

4. 调用接口：

```bash
curl -X POST http://127.0.0.1:28000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <uuid>" \
  -d '{
    "model": "glm-5",
    "messages": [{"role":"user","content":"你好"}]
  }'
```

## CLI 命令

```bash
iflow-go serve [--host] [--port] [--concurrency]
iflow-go token list
iflow-go token import
iflow-go token delete <uuid>
iflow-go token refresh <uuid>
iflow-go version
```

## 配置

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `IFLOW_HOST` | `0.0.0.0` | 服务监听地址 |
| `IFLOW_PORT` | `28000` | 服务监听端口 |
| `IFLOW_CONCURRENCY` | `1` | 并发数 |
| `IFLOW_DATA_DIR` | `./data` | 数据目录 |
| `IFLOW_LOG_LEVEL` | `info` | 日志级别（`debug`/`info`/`warn`/`error`） |
| `IFLOW_UPSTREAM_PROXY` | 空 | 上游代理 |

## 测试

```bash
go test ./...
go vet ./...
go test -cover ./...
```

当前总覆盖率：`74.7%`。

## 文档

- 架构：`docs/architecture.md`
- 实施计划：`docs/plan.md`
- API 文档：`docs/api.md`

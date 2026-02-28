# iflow-go

`iflow-go` 可以把 `iflow cli` 的请求转换为 OpenAI 兼容请求。

## 登录步骤

1. 使用 `iflow cli` OAuth 方式登录账号
2. 执行 `iflow-go token import ~/.iflow/settings.json` 导入账号，并获取该账号的 `API Key`
3. 启动服务 `iflow-go serve`
4. 接入 `cherry studio` `OpenClaw` 等应用

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

| 变量名                             | 默认值    | 说明                                                          |
| ---------------------------------- | --------- | ------------------------------------------------------------- |
| `IFLOW_HOST`                       | `0.0.0.0` | 服务监听地址                                                  |
| `IFLOW_PORT`                       | `28000`   | 服务监听端口                                                  |
| `IFLOW_CONCURRENCY`                | `1`       | 并发数                                                        |
| `IFLOW_DATA_DIR`                   | `./data`  | 数据目录                                                      |
| `IFLOW_LOG_LEVEL`                  | `info`    | 日志级别（`debug`/`info`/`warn`/`error`）                     |
| `IFLOW_UPSTREAM_PROXY`             | 空        | 上游代理                                                      |
| `IFLOW_PRESERVE_REASONING_CONTENT` | `true`    | 保留 `reasoning_content`，便于 Cherry Studio 等客户端展示思考 |

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

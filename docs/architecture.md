# iflow-go 项目架构

## 1. 项目概述

**iflow-go** 是 [iflow2api](https://github.com/cacaview/iflow2api) Python 项目的 Go 语言实现，提供 OpenAI 兼容的 API 代理服务。

### 核心特性
- 提供 `/v1/chat/completions` API 端点
- 支持多账号管理（通过 Bearer Token UUID 标识）
- 使用环境变量配置，无配置文件
- CLI 命令行管理，无 Web 后台

---

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              客户端请求                                       │
│              (Claude Code / OpenAI SDK / curl / ChatGPT-Next-Web)          │
│                                                                              │
│   Header: Authorization: Bearer <uuid>                                      │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          HTTP Server (net/http)                              │
│                                                                              │
│   Endpoints:                                                                 │
│   ├── GET  /health              健康检查                                    │
│   ├── GET  /v1/models           模型列表                                    │
│   └── POST /v1/chat/completions OpenAI 格式 Chat API                        │
│                                                                              │
│   Middleware:                                                                │
│   ├── 请求日志                                                               │
│   ├── Bearer Token 解析 → 获取 UUID → 加载账号配置                           │
│   └── 请求体大小限制                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          IFlowProxy (代理层)                                  │
│                                                                              │
│   功能:                                                                      │
│   1. 构造请求头 (User-Agent: iFlow-Cli, HMAC 签名)                          │
│   2. 配置模型特定参数                                                        │
│   3. 发送请求到 iFlow API                                                    │
│   4. 规范化响应 (reasoning_content 处理)                                    │
│   5. 发送遥测事件                                                            │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          iFlow API                                           │
│                       https://apis.iflow.cn/v1                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. 目录结构

```
iflow-go/
├── cmd/                        # CLI 入口
│   ├── root.go                 # 根命令
│   ├── serve.go                # serve 子命令
│   ├── token.go                # token 子命令组
│   └── version.go              # version 子命令
│
├── internal/                   # 内部包
│   ├── proxy/                  # 代理核心
│   │   ├── proxy.go            # IFlowProxy 主类
│   │   ├── headers.go          # 请求头构造
│   │   ├── signature.go        # HMAC 签名
│   │   ├── models.go           # 模型配置
│   │   └── telemetry.go        # 遥测上报
│   │
│   ├── account/                # 账号管理
│   │   ├── manager.go          # 账号管理器
│   │   ├── storage.go          # 存储操作
│   │   └── account.go          # 账号数据结构
│   │
│   ├── oauth/                  # OAuth 认证
│   │   ├── client.go           # OAuth 客户端、Token 交换与登录流程
│   │   └── refresher.go        # Token 自动刷新
│   │
│   ├── server/                 # HTTP 服务器
│   │   ├── server.go           # 服务器主类
│   │   ├── routes.go           # 路由定义
│   │   ├── middleware.go       # 中间件
│   │   ├── handlers.go         # 请求处理器
│   │   └── sse.go              # SSE 流式响应
│   │
│   └── config/                 # 配置管理
│       └── config.go           # 环境变量配置
│
├── pkg/                        # 可导出包 (可选)
│   └── types/                  # 公共类型定义
│       └── openai.go           # OpenAI API 类型
│
├── data/                       # 数据目录 (运行时创建)
│   └── accounts/               # 账号存储
│       ├── <uuid-1>.json       # 账号 1
│       └── <uuid-2>.json       # 账号 2
│
├── docs/                       # 文档
│   ├── research/               # 调研文档
│   ├── archive/                # 已完成计划归档
│   ├── architecture.md         # 项目架构
│   └── plan.md                 # 实施计划
│
├── AGENTS.md                   # 开发规则与阶段
├── main.go                     # 程序入口
├── go.mod
├── go.sum
├── Makefile
└── README.md                   # (待创建)
```

---

## 4. 数据结构

### 4.1 账号配置 (Account)

**存储路径**: `$IFLOW_DATA_DIR/accounts/<uuid>.json`

```json
{
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "api_key": "sk-xxx",
  "base_url": "https://apis.iflow.cn/v1",
  "auth_type": "oauth-iflow",
  "oauth_access_token": "xxx",
  "oauth_refresh_token": "xxx",
  "oauth_expires_at": "2024-12-31T23:59:59Z",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z",
  "last_used_at": "2024-01-15T10:30:00Z",
  "request_count": 1234
}
```

### 4.2 环境变量配置

```bash
# 服务配置
IFLOW_HOST=0.0.0.0              # 监听地址
IFLOW_PORT=28000                # 监听端口
IFLOW_CONCURRENCY=1             # 并发数

# 数据目录
IFLOW_DATA_DIR=./data           # 数据存储目录

# 上游代理 (可选)
IFLOW_UPSTREAM_PROXY=           # 代理地址 (http:// 或 socks5://)

# 日志级别
IFLOW_LOG_LEVEL=info            # debug, info, warn, error
```

---

## 5. API 端点

### 5.1 服务端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/v1/models` | GET | 获取可用模型列表 |
| `/v1/chat/completions` | POST | OpenAI 格式 Chat API |

### 5.2 请求头处理

```
客户端请求:
  Authorization: Bearer <uuid>

处理流程:
  1. 解析 Bearer Token 获取 UUID
  2. 加载 $IFLOW_DATA_DIR/accounts/<uuid>.json
  3. 使用该账号的 API Key 构造上游请求
```

### 5.3 上游请求头

```http
Content-Type: application/json
Authorization: Bearer <api_key>
user-agent: iFlow-Cli
session-id: session-<uuid>
conversation-id: <conversation-uuid>
x-iflow-signature: <hmac-sha256>
x-iflow-timestamp: <milliseconds>
traceparent: <w3c-trace-context>
```

---

## 6. CLI 命令

```
iflow-go
├── serve              启动代理服务
│   ├── --host         监听地址 (默认: 0.0.0.0)
│   ├── --port         监听端口 (默认: 28000)
│   └── --concurrency  并发数 (默认: 1)
│
├── token              Token 管理
│   ├── list           列出所有账号
│   ├── import         导入账号 (交互式 OAuth 登录)
│   ├── delete <uuid>  删除账号
│   └── refresh <uuid> 手动刷新 Token
│
└── version            显示版本信息
```

---

## 7. 核心流程

### 7.1 请求处理流程

```
1. 接收请求 → 解析 Bearer Token (UUID)
                │
                ▼
2. 加载账号配置 → $IFLOW_DATA_DIR/accounts/<uuid>.json
                │
                ├── 找到 → 继续
                └── 未找到 → 返回 401 Unauthorized
                │
                ▼
3. 检查 Token 是否过期
                │
                ├── 已过期 → 尝试刷新 → 成功 → 继续
                │                    └── 失败 → 返回 401
                └── 未过期 → 继续
                │
                ▼
4. 构造上游请求
   - 添加 iFlow-Cli User-Agent
   - 生成 HMAC 签名
   - 配置模型特定参数
                │
                ▼
5. 发送请求到 iFlow API
                │
                ▼
6. 规范化响应
   - 处理 reasoning_content
   - 统一响应格式
                │
                ▼
7. 返回响应给客户端
```

### 7.2 Token 刷新流程

```
定时任务 (每 6 小时):
  │
  ▼
遍历所有账号
  │
  ▼
检查是否即将过期 (< 24 小时)
  │
  ├── 是 → 刷新 Token
  │         │
  │         ├── 成功 → 保存新 Token
  │         └── 失败 → 标记账号无效
  │
  └── 否 → 跳过
```

---

## 8. 错误处理

### 8.1 错误码

| HTTP 状态码 | 说明 |
|-------------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 认证失败 (无效 UUID 或 Token 过期) |
| 429 | 请求过于频繁 |
| 500 | 服务器内部错误 |
| 502 | 上游服务错误 |

### 8.2 错误响应格式

```json
{
  "error": {
    "message": "错误描述",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

---

## 9. 依赖库

```go
// go.mod
module github.com/rogeecn/iflow-go

go 1.21

require (
    github.com/spf13/cobra v1.8.0           // CLI 框架
    github.com/joho/godotenv v1.5.1         // 环境变量加载
    github.com/google/uuid v1.6.0           // UUID 生成
    github.com/rs/zerolog v1.32.0           // 结构化日志
    golang.org/x/oauth2 v0.18.0             // OAuth2 客户端
    github.com/caarlos0/env/v10 v10.0.0     // 环境变量解析
)
```

---

## 10. 与 Python 版本的差异

| 功能 | Python 版本 | Go 版本 |
|------|-------------|---------|
| Web 管理界面 | ✅ FastAPI + Admin UI | ❌ 仅 CLI |
| 配置存储 | 文件 + 加密 | 环境变量 |
| Token 存储 | 加密存储 | 明文 JSON |
| 多账号 | 实例管理 | UUID 文件 |
| GUI | ✅ Flet | ❌ 无 |
| 系统托盘 | ✅ pystray | ❌ 无 |
| TLS 指纹伪装 | ✅ curl_cffi | ❌ 标准库 |
| 自动更新 | ✅ | ❌ |
| 国际化 | ✅ | ❌ |

---

## 11. 文档更新记录

| 日期 | 版本 | 变更说明 |
|------|------|----------|
| 2026-02-27 | v1.0 | 初始版本，基于 Python 项目调研 |
| 2026-02-28 | v1.1 | 同步 OAuth 目录结构，明确 `client.go` 承担 Token 交换与登录流程 |
| 2026-02-28 | v1.2 | 补充程序入口 `main.go`，对齐 Phase 6 CLI 实现结构 |

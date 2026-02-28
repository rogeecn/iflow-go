# iflow2api 架构概述

## 1. 项目简介

**iflow2api** 是一个将 iFlow CLI 的 AI 服务暴露为 OpenAI 兼容 API 的代理服务。

### 核心功能
- 提供 OpenAI 兼容的 `/v1/chat/completions` 端点
- 提供 Anthropic 兼容的 `/v1/messages` 端点（Claude Code 兼容）
- 通过 `User-Agent: iFlow-Cli` 解锁 CLI 专属高级模型
- 支持 OAuth 认证和 Token 自动刷新
- 支持多账号管理

### 技术栈 (Python)
- **HTTP Server**: FastAPI + Uvicorn
- **HTTP Client**: httpx + curl_cffi (TLS 指纹伪装)
- **CLI**: argparse
- **GUI**: Flet (Flutter-based)
- **Desktop**: pystray (系统托盘)

---

## 2. 目录结构

```
iflow2api/
├── __init__.py              # 包初始化
├── __main__.py              # CLI 入口
├── main.py                  # 主入口
├── app.py                   # FastAPI 应用 (核心路由)
├── config.py                # iFlow 配置读取 (~/.iflow/settings.json)
├── proxy.py                 # API 代理 (添加 User-Agent header, 签名)
├── transport.py             # 统一传输层 (httpx / curl_cffi)
├── oauth.py                 # OAuth 认证逻辑
├── oauth_login.py           # OAuth 登录处理器
├── token_refresher.py       # OAuth token 自动刷新
├── settings.py              # 应用配置管理 (~/.iflow2api/config.json)
├── vision.py                # Vision 支持 (图像处理)
├── crypto.py                # 配置加密
├── instances.py             # 多实例管理
├── server.py                # 服务管理 (后台线程运行 uvicorn)
├── ratelimit.py             # 速率限制
├── i18n.py                  # 国际化
├── tray.py                  # 系统托盘
├── autostart.py             # 开机自启动
├── updater.py               # 自动更新
├── gui.py                   # GUI 界面
├── web_server.py            # Web 服务器
└── admin/                   # Web 管理界面
    ├── __init__.py
    ├── auth.py              # 管理界面认证
    ├── routes.py            # 管理界面路由
    ├── websocket.py         # WebSocket 通信
    └── static/              # 静态文件
```

---

## 3. 核心模块交互图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              客户端请求                                       │
│         (Claude Code / OpenAI SDK / curl / ChatGPT-Next-Web)               │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          FastAPI Application (app.py)                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Endpoints:                                                          │   │
│  │  - /v1/chat/completions (OpenAI 格式)                               │   │
│  │  - /v1/messages (Anthropic 格式)                                    │   │
│  │  - /v1/models                                                        │   │
│  │  - /health                                                           │   │
│  │  - /admin/* (管理界面)                                               │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Middleware:                                                         │   │
│  │  - CORS                                                              │   │
│  │  - 请求体大小限制 (10MB)                                             │   │
│  │  - 自定义 API 鉴权                                                   │   │
│  │  - 请求日志                                                          │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          IFlowProxy (proxy.py)                               │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  功能:                                                               │   │
│  │  1. 添加 iFlow-Cli User-Agent                                        │   │
│  │  2. 生成 HMAC-SHA256 签名                                            │   │
│  │  3. 添加 session-id, conversation-id, traceparent                    │   │
│  │  4. 发送 telemetry 事件                                              │   │
│  │  5. 规范化响应 (reasoning_content 处理)                              │   │
│  │  6. 模型特定参数配置                                                  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Transport Layer (transport.py)                            │
│  ┌─────────────────────────────┐    ┌─────────────────────────────────┐    │
│  │  HttpxTransport             │    │  CurlCffiTransport              │    │
│  │  - 标准 TLS                 │    │  - TLS 指纹伪装 (chrome124)     │    │
│  │  - 支持 SSE 流式            │    │  - 支持 SSE 流式                │    │
│  │  - 代理支持                 │    │  - 代理支持                     │    │
│  └─────────────────────────────┘    └─────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            iFlow API                                         │
│                       https://apis.iflow.cn/v1                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. 配置文件

### 4.1 iFlow CLI 配置
**路径**: `~/.iflow/settings.json`

```json
{
  "apiKey": "xxx",
  "baseUrl": "https://apis.iflow.cn/v1",
  "selectedAuthType": "oauth-iflow",
  "oauth_access_token": "xxx",
  "oauth_refresh_token": "xxx",
  "oauth_expires_at": "2024-xx-xxTxx:xx:xx"
}
```

### 4.2 应用配置
**路径**: `~/.iflow2api/config.json`

```json
{
  "host": "0.0.0.0",
  "port": 28000,
  "api_key": "xxx",
  "base_url": "https://apis.iflow.cn/v1",
  "auth_type": "oauth-iflow",
  "preserve_reasoning_content": true,
  "api_concurrency": 1,
  "custom_api_key": "",
  "upstream_proxy": "",
  "upstream_proxy_enabled": false,
  "upstream_transport_backend": "curl_cffi",
  "tls_impersonate": "chrome124"
}
```

---

## 5. 认证类型

| 类型 | 说明 |
|------|------|
| `oauth-iflow` | iFlow OAuth 登录 (推荐) |
| `api-key` | 直接使用 API Key |
| `openai-compatible` | OpenAI 兼容模式 |

---

## 6. 支持的模型

### 文本模型
- `glm-4.6`, `glm-4.7`, `glm-5` (智谱)
- `deepseek-v3.2-chat` (DeepSeek)
- `qwen3-coder-plus` (通义千问)
- `kimi-k2`, `kimi-k2-thinking`, `kimi-k2.5` (Moonshot)
- `minimax-m2.5` (MiniMax)
- `iFlow-ROME-30BA3B` (iFlow)

### 视觉模型
- `qwen-vl-max` (通义千问 VL)
- `glm-4v`, `glm-4v-plus`, `glm-4v-flash`, `glm-4.5v`, `glm-4.6v`

---

## 7. API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/v1/models` | GET | 获取可用模型列表 |
| `/v1/chat/completions` | POST | OpenAI 格式 Chat API |
| `/v1/messages` | POST | Anthropic 格式 Messages API |
| `/v1/vision-models` | GET | 获取视觉模型列表 |
| `/admin` | GET | Web 管理界面 |
| `/admin/login` | POST | 管理界面登录 |
| `/admin/settings` | GET/PUT | 应用设置 |
| `/admin/oauth/url` | GET | 获取 OAuth 登录 URL |
| `/admin/oauth/callback` | GET/POST | OAuth 回调 |
| `/admin/server/start` | POST | 启动服务器 |
| `/admin/server/stop` | POST | 停止服务器 |
| `/admin/ws` | WebSocket | 实时状态推送 |
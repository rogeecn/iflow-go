# iflow-go API 文档

## 概览

服务提供 OpenAI 兼容接口：

- `GET /health`
- `GET /v1/models`
- `POST /v1/chat/completions`

除 `/health` 外，其余端点都需要：

```http
Authorization: Bearer <uuid>
```

其中 `<uuid>` 对应本地账号文件 `data/accounts/<uuid>.json`。

## 1. 健康检查

### 请求

```http
GET /health
```

### 响应

```json
{
  "status": "ok"
}
```

## 2. 获取模型列表

### 请求

```http
GET /v1/models
Authorization: Bearer <uuid>
```

### 响应

```json
{
  "object": "list",
  "data": [
    {
      "id": "glm-5",
      "object": "model",
      "created": 1700000000,
      "owned_by": "iflow",
      "permission": [],
      "root": "glm-5",
      "parent": null
    }
  ]
}
```

## 3. Chat Completions

### 非流式请求

```http
POST /v1/chat/completions
Content-Type: application/json
Authorization: Bearer <uuid>
```

```json
{
  "model": "glm-5",
  "messages": [
    {
      "role": "user",
      "content": "你好"
    }
  ]
}
```

### 非流式响应

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "glm-5",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好，有什么可以帮你？"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 0,
    "completion_tokens": 0,
    "total_tokens": 0
  }
}
```

### 流式请求

```json
{
  "model": "glm-5",
  "stream": true,
  "messages": [
    {
      "role": "user",
      "content": "写一个 Go HTTP 服务示例"
    }
  ]
}
```

### 流式响应

```text
data: {"id":"chunk-1","choices":[{"delta":{"content":"第一段"}}]}

data: {"id":"chunk-2","choices":[{"delta":{"content":"第二段"}}]}

data: [DONE]
```

## 4. 错误响应

统一错误格式：

```json
{
  "error": {
    "message": "错误描述",
    "type": "invalid_request_error",
    "code": "bad_request"
  }
}
```

常见状态码：

| 状态码 | 含义 |
|---|---|
| `400` | 请求体错误或缺少必要字段 |
| `401` | Bearer Token 无效 |
| `413` | 请求体过大 |
| `502` | 上游请求失败 |

## 5. 兼容性说明

- 对于只返回 `reasoning_content` 的上游模型，服务会自动归一化到 `content`
- 流式响应也会做同样的兼容处理
- 默认保留 `reasoning_content` 字段（`IFLOW_PRESERVE_REASONING_CONTENT=true`），且不会再镜像到 `content`，便于 Cherry Studio 展示独立思考过程
- 若需兼容仅识别 `content` 的客户端，可设置 `IFLOW_PRESERVE_REASONING_CONTENT=false`
- `/v1/models` 返回本地内置模型清单，不依赖上游 `/models` 接口

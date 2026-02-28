# IFlowProxy 模块详解

**文件**: `iflow2api/proxy.py`

## 1. 核心职责

IFlowProxy 是与 iFlow API 交互的核心代理类，负责：

1. **请求构造**: 添加 iFlow CLI 特有的请求头和签名
2. **流量转发**: 转发 chat completions 请求到 iFlow API
3. **响应处理**: 规范化 OpenAI 格式响应
4. **遥测上报**: 发送使用统计到 mmstat

---

## 2. 类结构

```python
class IFlowProxy:
    def __init__(self, config: IFlowConfig):
        self.config = config                    # 配置
        self.base_url = config.base_url         # API 基础 URL
        self._client: BaseUpstreamTransport     # HTTP 客户端
        self._session_id: str                   # 会话 ID
        self._conversation_id: str              # 对话 ID
        self._telemetry_user_id: str            # 遥测用户 ID
```

---

## 3. 关键请求头

```python
def _get_headers(self, stream: bool = False, traceparent: Optional[str] = None) -> dict:
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {self.config.api_key}",
        "user-agent": "iFlow-Cli",              # 小写键名
        "session-id": self._session_id,         # session-{uuid}
        "conversation-id": self._conversation_id,
        "accept": "*/*",
        "accept-language": "*",
        "sec-fetch-mode": "cors",
        "accept-encoding": "br, gzip, deflate",
        "x-iflow-signature": signature,         # HMAC-SHA256 签名
        "x-iflow-timestamp": str(timestamp),    # 毫秒时间戳
        "traceparent": traceparent,             # W3C Trace Context
    }
    
    # Aone 分支专有头 (ducky.code.alibaba-inc.com)
    if self._is_aone_endpoint():
        headers["X-Client-Type"] = "iflow-cli"
        headers["X-Client-Version"] = "0.5.13"
    
    return headers
```

---

## 4. HMAC-SHA256 签名算法

```python
def generate_signature(user_agent: str, session_id: str, timestamp: int, api_key: str) -> str:
    """
    签名内容: "{user_agent}:{session_id}:{timestamp}"
    算法: HMAC-SHA256
    密钥: api_key
    输出: 十六进制字符串
    """
    message = f"{user_agent}:{session_id}:{timestamp}"
    return hmac.new(
        api_key.encode('utf-8'),
        message.encode('utf-8'),
        hashlib.sha256
    ).hexdigest()
```

**Go 实现**:
```go
func generateSignature(userAgent, sessionID string, timestamp int64, apiKey string) string {
    message := fmt.Sprintf("%s:%s:%d", userAgent, sessionID, timestamp)
    h := hmac.New(sha256.New, []byte(apiKey))
    h.Write([]byte(message))
    return hex.EncodeToString(h.Sum(nil))
}
```

---

## 5. 模型特定参数配置

```python
def _configure_model_request(request_body: dict, model: str) -> dict:
    """
    为特定模型添加必要的请求参数
    """
    model_lower = model.lower()
    
    # DeepSeek
    if model_lower.startswith("deepseek"):
        body["thinking_mode"] = True
        body["reasoning"] = True
    
    # GLM-5
    elif model == "glm-5":
        body["chat_template_kwargs"] = {"enable_thinking": True}
        body["enable_thinking"] = True
        body["thinking"] = {"type": "enabled"}
    
    # GLM-4.7 / 其他 GLM
    elif model_lower.startswith("glm-"):
        body["chat_template_kwargs"] = {"enable_thinking": True}
    
    # Kimi-K2.5
    elif model_lower.startswith("kimi-k2.5"):
        body["thinking"] = {"type": "enabled"}
    
    # thinking 模型 (kimi-k2-thinking, gemini-2.0-flash-thinking)
    elif "thinking" in model_lower:
        body["thinking_mode"] = True
    
    return body
```

---

## 6. 默认请求参数

```python
def _align_official_body_defaults(request_body: dict, stream: bool = False) -> dict:
    """
    对齐 iflow-cli 的默认参数
    """
    body.pop("stream", None)  # 先移除
    if stream:
        body["stream"] = True
    
    # 默认参数
    body.setdefault("temperature", 0.7)
    body.setdefault("top_p", 0.95)
    body.setdefault("max_new_tokens", 8192)
    body.setdefault("tools", [])
    
    return body
```

---

## 7. 响应规范化

```python
def _normalize_response(result: dict, preserve_reasoning: bool = False) -> dict:
    """
    处理 reasoning_content 字段
    
    某些模型 (GLM-5) 使用 reasoning_content 返回思考过程，
    但 OpenAI 兼容客户端只检查 content 字段。
    """
    for choice in result.get("choices", []):
        message = choice.get("message", {})
        content = message.get("content")
        reasoning_content = message.get("reasoning_content")
        
        if not content and reasoning_content:
            # 将 reasoning_content 移动到 content
            message["content"] = reasoning_content
            if not preserve_reasoning:
                del message["reasoning_content"]
    
    return result
```

---

## 8. 遥测事件

```python
# 发送 run_started 事件
async def _emit_run_started(self, model: str, trace_id: str) -> str:
    observation_id = self._rand_observation_id()
    gokey = (
        f"pid=iflow"
        f"&sam=iflow.cli.{self._conversation_id}.{trace_id}"
        f"&trace_id={trace_id}"
        f"&session_id={self._session_id}"
        f"&conversation_id={self._conversation_id}"
        f"&observation_id={observation_id}"
        f"&model={urllib.parse.quote(model)}"
        f"&user_id={self._telemetry_user_id}"
    )
    await self._telemetry_post_gm("//aitrack.lifecycle.run_started", gokey)
    return observation_id

# 发送 run_error 事件
async def _emit_run_error(self, model: str, trace_id: str, 
                          parent_observation_id: str, error_msg: str):
    # ... 类似结构，增加错误信息
```

---

## 9. 流式响应处理

```python
async def chat_completions(self, request_body: dict, stream: bool = False):
    if stream:
        async def stream_generator():
            async with client.stream("POST", f"{self.base_url}/chat/completions",
                                     headers=headers, json_body=request_body) as response:
                response.raise_for_status()
                
                buffer = b""
                async for chunk in response.aiter_bytes():
                    buffer += chunk
                    while b"\n" in buffer:
                        line, buffer = buffer.split(b"\n", 1)
                        line_str = line.decode("utf-8").strip()
                        
                        if line_str.startswith("data:"):
                            data_str = line_str[5:].strip()
                            if data_str == "[DONE]":
                                yield b"data: [DONE]\n\n"
                                continue
                            
                            chunk_data = json.loads(data_str)
                            chunk_data = self._normalize_stream_chunk(chunk_data)
                            yield ("data: " + json.dumps(chunk_data) + "\n\n").encode()
        
        return stream_generator()
    else:
        # 非流式
        response = await client.post(...)
        return self._normalize_response(response.json())
```

---

## 10. 模型列表

```python
MODELS = [
    # 文本模型
    {"id": "glm-4.6", "name": "GLM-4.6", "description": "智谱 GLM-4.6"},
    {"id": "glm-4.7", "name": "GLM-4.7", "description": "智谱 GLM-4.7"},
    {"id": "glm-5", "name": "GLM-5", "description": "智谱 GLM-5 (推荐)"},
    {"id": "iFlow-ROME-30BA3B", "name": "iFlow-ROME-30BA3B", "description": "iFlow ROME 30B"},
    {"id": "deepseek-v3.2-chat", "name": "DeepSeek-V3.2", "description": "DeepSeek V3.2"},
    {"id": "qwen3-coder-plus", "name": "Qwen3-Coder-Plus", "description": "通义千问 Coder"},
    {"id": "kimi-k2", "name": "Kimi-K2", "description": "Moonshot Kimi K2"},
    {"id": "kimi-k2-thinking", "name": "Kimi-K2-Thinking", "description": "Kimi K2 思考模型"},
    {"id": "kimi-k2.5", "name": "Kimi-K2.5", "description": "Moonshot Kimi K2.5"},
    {"id": "kimi-k2-0905", "name": "Kimi-K2-0905", "description": "Moonshot Kimi K2 0905"},
    {"id": "minimax-m2.5", "name": "MiniMax-M2.5", "description": "MiniMax M2.5"},
    
    # 视觉模型
    {"id": "qwen-vl-max", "name": "Qwen-VL-Max", "description": "通义千问 VL Max"},
]
```

---

## 11. Go 实现要点

### 11.1 结构体定义

```go
type IFlowProxy struct {
    config          *IFlowConfig
    baseURL         string
    client          HTTPClient
    sessionID       string
    conversationID  string
    telemetryUserID string
}

type IFlowConfig struct {
    APIKey             string
    BaseURL            string
    AuthType           string
    OAuthAccessToken   string
    OAuthRefreshToken  string
    OAuthExpiresAt     time.Time
}
```

### 11.2 请求头构造

```go
func (p *IFlowProxy) getHeaders(stream bool, traceparent string) map[string]string {
    timestamp := time.Now().UnixMilli()
    signature := generateSignature("iFlow-Cli", p.sessionID, timestamp, p.config.APIKey)
    
    headers := map[string]string{
        "Content-Type":      "application/json",
        "Authorization":     "Bearer " + p.config.APIKey,
        "user-agent":        "iFlow-Cli",
        "session-id":        p.sessionID,
        "conversation-id":   p.conversationID,
        "accept":            "*/*",
        "accept-language":   "*",
        "sec-fetch-mode":    "cors",
        "accept-encoding":   "br, gzip, deflate",
        "x-iflow-signature": signature,
        "x-iflow-timestamp": fmt.Sprintf("%d", timestamp),
        "traceparent":       traceparent,
    }
    
    return headers
}
```

### 11.3 流式响应

```go
func (p *IFlowProxy) ChatCompletionsStream(ctx context.Context, req *ChatCompletionRequest) (<-chan []byte, error) {
    // 使用 Go 的 SSE 或手动解析
    resp, err := p.client.Do(req)
    if err != nil {
        return nil, err
    }
    
    ch := make(chan []byte, 100)
    go func() {
        defer close(ch)
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            line := scanner.Text()
            if strings.HasPrefix(line, "data:") {
                data := strings.TrimPrefix(line, "data:")
                data = strings.TrimSpace(data)
                if data == "[DONE]" {
                    ch <- []byte("data: [DONE]\n\n")
                    return
                }
                // 处理并规范化
                normalized := p.normalizeStreamChunk(data)
                ch <- []byte("data: " + normalized + "\n\n")
            }
        }
    }()
    
    return ch, nil
}
```
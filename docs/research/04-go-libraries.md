# Go 库选型

## 1. HTTP 服务器

### 推荐方案: 标准库 `net/http` + `gorilla/mux` 或 `chi`

| 库 | 优点 | 缺点 |
|---|---|---|
| `net/http` (标准库) | 零依赖、稳定、SSE 支持 | 路由功能有限 |
| `github.com/go-chi/chi/v5` | 轻量、中间件链、路由分组 | 需额外引入 |
| `github.com/gorilla/mux` | 功能丰富、社区成熟 | 相对重量级 |

**SSE 流式响应示例**:
```go
func streamHandler(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    
    for {
        select {
        case <-r.Context().Done():
            return
        case data := <-dataChan:
            fmt.Fprintf(w, "data: %s\n\n", data)
            flusher.Flush()
        }
    }
}
```

---

## 2. HTTP 客户端

### 推荐方案: 标准库 `net/http` + 代理支持

| 库 | 功能 | 推荐度 |
|---|---|---|
| `net/http` (标准库) | 流式响应、代理、连接池 | ⭐⭐⭐⭐⭐ |
| `github.com/hashicorp/go-retryablehttp` | 自动重试 | ⭐⭐⭐ |

**TLS 指纹伪装问题**:
- Python 的 `curl_cffi` 使用 curl-impersonate 伪装 TLS 指纹
- Go 标准库使用原生 TLS，无法伪装
- **解决方案**: Go 标准库通常足够，iFlow API 未强制校验 TLS 指纹

**流式请求示例**:
```go
func streamingRequest(ctx context.Context, url string, body io.Reader) (<-chan []byte, error) {
    req, err := http.NewRequestWithContext(ctx, "POST", url, body)
    if err != nil {
        return nil, err
    }
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    
    ch := make(chan []byte, 100)
    go func() {
        defer close(ch)
        defer resp.Body.Close()
        
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            ch <- scanner.Bytes()
        }
    }()
    
    return ch, nil
}
```

---

## 3. OAuth2 客户端

### 推荐方案: `golang.org/x/oauth2`

```go
import "golang.org/x/oauth2"

// iFlow OAuth 配置
var iflowOAuthConfig = &oauth2.Config{
    ClientID:     "10009311001",
    ClientSecret: "4Z3YjXycVsQvyGF1etiNlIBB4RsqSDtW",
    Endpoint: oauth2.Endpoint{
        AuthURL:  "https://iflow.cn/oauth",
        TokenURL: "https://iflow.cn/oauth/token",
    },
    RedirectURL: "http://localhost:28000/oauth/callback",
    Scopes:      []string{},
}

// 生成授权 URL (带 PKCE)
func getAuthURL() (string, string) {
    verifier := oauth2.GenerateVerifier()
    state := generateRandomState()
    
    url := iflowOAuthConfig.AuthCodeURL(state,
        oauth2.AccessTypeOffline,
        oauth2.S256ChallengeOption(verifier),
    )
    
    return url, verifier
}

// 使用授权码换取 Token
func exchangeToken(ctx context.Context, code, verifier string) (*oauth2.Token, error) {
    return iflowOAuthConfig.Exchange(ctx, code,
        oauth2.VerifierOption(verifier),
    )
}

// 刷新 Token
func refreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
    token := &oauth2.Token{
        RefreshToken: refreshToken,
    }
    
    ts := iflowOAuthConfig.TokenSource(ctx, token)
    return ts.Token()
}
```

---

## 4. CLI 框架

### 推荐方案: `github.com/spf13/cobra`

```go
import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
    Use:   "iflow-go",
    Short: "iFlow API 代理服务",
}

// serve 子命令
var serveCmd = &cobra.Command{
    Use:   "serve",
    Short: "启动代理服务",
    Run:   runServe,
}

// token 子命令组
var tokenCmd = &cobra.Command{
    Use:   "token",
    Short: "Token 管理",
}

var tokenListCmd = &cobra.Command{
    Use:   "list",
    Short: "列出所有 Token",
    Run:   listTokens,
}

var tokenImportCmd = &cobra.Command{
    Use:   "import <uuid>",
    Short: "导入 Token",
    Args:  cobra.ExactArgs(1),
    Run:   importToken,
}

var tokenDeleteCmd = &cobra.Command{
    Use:   "delete <uuid>",
    Short: "删除 Token",
    Args:  cobra.ExactArgs(1),
    Run:   deleteToken,
}

var tokenRefreshCmd = &cobra.Command{
    Use:   "refresh <uuid>",
    Short: "刷新 Token",
    Args:  cobra.ExactArgs(1),
    Run:   refreshToken,
}

func init() {
    rootCmd.AddCommand(serveCmd)
    rootCmd.AddCommand(tokenCmd)
    tokenCmd.AddCommand(tokenListCmd, tokenImportCmd, tokenDeleteCmd, tokenRefreshCmd)
}
```

**命令结构**:
```
iflow-go
├── serve              # 启动代理服务
│   ├── --port         # 监听端口
│   ├── --host         # 监听地址
│   └── --concurrency  # 并发数
├── token              # Token 管理
│   ├── list           # 列出所有 Token
│   ├── import <uuid>  # 导入 Token (交互式)
│   ├── delete <uuid>  # 删除 Token
│   └── refresh <uuid> # 刷新 Token
└── version            # 版本信息
```

---

## 5. 环境变量

### 推荐方案: `github.com/joho/godotenv`

```go
import "github.com/joho/godotenv"

func init() {
    godotenv.Load() // 加载 .env 文件
}

// 环境变量配置
type Config struct {
    Host        string `env:"IFLOW_HOST" envDefault:"0.0.0.0"`
    Port        int    `env:"IFLOW_PORT" envDefault:"28000"`
    Concurrency int    `env:"IFLOW_CONCURRENCY" envDefault:"1"`
    DataDir     string `env:"IFLOW_DATA_DIR" envDefault:"./data"`
}
```

**环境变量列表**:
```bash
# 服务配置
IFLOW_HOST=0.0.0.0
IFLOW_PORT=28000
IFLOW_CONCURRENCY=1

# 数据目录
IFLOW_DATA_DIR=./data

# 上游代理 (可选)
IFLOW_UPSTREAM_PROXY=http://127.0.0.1:7890
```

---

## 6. 其他工具库

| 功能 | 库 | 说明 |
|------|-----|------|
| UUID 生成 | `github.com/google/uuid` | 生成 Token UUID |
| JSON 处理 | `encoding/json` | 标准库 |
| 日志 | `github.com/rs/zerolog` | 结构化日志 |
| 配置 | `github.com/caarlos0/env/v10` | 环境变量解析 |

---

## 7. 项目依赖总结

```go
// go.mod
module github.com/rogeecn/iflow-go

go 1.21

require (
    github.com/spf13/cobra v1.8.0
    github.com/joho/godotenv v1.5.1
    github.com/google/uuid v1.6.0
    github.com/rs/zerolog v1.32.0
    golang.org/x/oauth2 v0.18.0
    github.com/caarlos0/env/v10 v10.0.0
)
```
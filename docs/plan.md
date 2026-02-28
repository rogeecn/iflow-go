# iflow-go 实施计划

> **规则**: 本计划必须严格按照阶段顺序执行。所有阶段完成后，将计划归档到 `docs/archive/` 目录。

---

## 阶段概览

```
Phase 0: 项目初始化     [x] 已完成
    │
    ▼
Phase 1: 核心基础设施   [x] 已完成
    │
    ▼
Phase 2: 账号管理       [x] 已完成
    │
    ▼
Phase 3: OAuth 认证     [x] 已完成
    │
    ▼
Phase 4: API 代理       [x] 已完成
    │
    ▼
Phase 5: HTTP 服务器    [x] 已完成
    │
    ▼
Phase 6: CLI 完善       [x] 已完成
    │
    ▼
Phase 7: 测试与文档     [x] 已完成
    │
    ▼
Phase 8: 官方请求对齐   [x] 已完成
```

---

## Phase 0: 项目初始化

**目标**: 创建项目骨架，配置开发环境

### 任务清单

- [x] **P0-1** 初始化 Go 模块
  ```bash
  go mod init github.com/rogeecn/iflow-go
  ```

- [x] **P0-2** 创建目录结构
  ```
  mkdir -p cmd internal/proxy internal/account internal/oauth internal/server internal/config pkg/types data/accounts docs/archive
  ```

- [x] **P0-3** 创建 Makefile
  ```makefile
  .PHONY: build run test clean
  
  build:
      go build -o bin/iflow-go ./cmd
  
  run:
      go run ./cmd
  
  test:
      go test ./... -v
  
  clean:
      rm -rf bin/
  ```

- [x] **P0-4** 创建 .env.example
  ```bash
  IFLOW_HOST=0.0.0.0
  IFLOW_PORT=28000
  IFLOW_CONCURRENCY=1
  IFLOW_DATA_DIR=./data
  IFLOW_LOG_LEVEL=info
  ```

- [x] **P0-5** 创建 .gitignore
  ```
  bin/
  data/
  .env
  ```

- [x] **P0-6** 安装依赖
  ```bash
  go get github.com/spf13/cobra@latest
  go get github.com/joho/godotenv@latest
  go get github.com/google/uuid@latest
  go get github.com/rs/zerolog@latest
  go get golang.org/x/oauth2@latest
  go get github.com/caarlos0/env/v10@latest
  ```
  > 说明 (2026-02-27): 当前环境禁止联网，已使用本地模块缓存离线安装固定版本并通过验收。

### 验收标准
- [x] `go mod tidy` 无错误
- [x] 目录结构符合架构文档
- [x] `make build` 成功生成可执行文件

---

## Phase 1: 核心基础设施

**目标**: 实现配置管理和基础工具函数

### 任务清单

- [x] **P1-1** 实现环境变量配置 (`internal/config/config.go`)
  ```go
  type Config struct {
      Host        string `env:"IFLOW_HOST" envDefault:"0.0.0.0"`
      Port        int    `env:"IFLOW_PORT" envDefault:"28000"`
      Concurrency int    `env:"IFLOW_CONCURRENCY" envDefault:"1"`
      DataDir     string `env:"IFLOW_DATA_DIR" envDefault:"./data"`
      LogLevel    string `env:"IFLOW_LOG_LEVEL" envDefault:"info"`
      Proxy       string `env:"IFLOW_UPSTREAM_PROXY"`
  }
  
  func Load() (*Config, error)
  ```

- [x] **P1-2** 实现日志初始化
  ```go
  func InitLogger(level string) zerolog.Logger
  ```

- [x] **P1-3** 实现 UUID 工具函数
  ```go
  func GenerateUUID() string
  func IsValidUUID(s string) bool
  ```

- [x] **P1-4** 创建公共类型定义 (`pkg/types/openai.go`)
  - ChatCompletionRequest, ChatCompletionResponse
  - Message, Choice, Delta, Usage, etc.

### 验收标准
- [x] 配置可从环境变量正确加载
- [x] 日志输出格式正确
- [x] 类型定义与 OpenAI API 规范一致

---

## Phase 2: 账号管理

**目标**: 实现多账号存储与管理

### 任务清单

- [x] **P2-1** 定义账号数据结构 (`internal/account/account.go`)
  ```go
  type Account struct {
      UUID              string    `json:"uuid"`
      APIKey            string    `json:"api_key"`
      BaseURL           string    `json:"base_url"`
      AuthType          string    `json:"auth_type"`
      OAuthAccessToken  string    `json:"oauth_access_token,omitempty"`
      OAuthRefreshToken string    `json:"oauth_refresh_token,omitempty"`
      OAuthExpiresAt    time.Time `json:"oauth_expires_at,omitempty"`
      CreatedAt         time.Time `json:"created_at"`
      UpdatedAt         time.Time `json:"updated_at"`
      LastUsedAt        time.Time `json:"last_used_at,omitempty"`
      RequestCount      int       `json:"request_count"`
  }
  ```

- [x] **P2-2** 实现存储操作 (`internal/account/storage.go`)
  ```go
  type Storage struct {
      dataDir string
  }
  
  func NewStorage(dataDir string) *Storage
  func (s *Storage) Save(account *Account) error
  func (s *Storage) Load(uuid string) (*Account, error)
  func (s *Storage) Delete(uuid string) error
  func (s *Storage) List() ([]*Account, error)
  func (s *Storage) Exists(uuid string) bool
  ```

- [x] **P2-3** 实现账号管理器 (`internal/account/manager.go`)
  ```go
  type Manager struct {
      storage *Storage
  }
  
  func NewManager(dataDir string) *Manager
  func (m *Manager) Create(apiKey, baseURL string) (*Account, error)
  func (m *Manager) Get(uuid string) (*Account, error)
  func (m *Manager) Delete(uuid string) error
  func (m *Manager) List() ([]*Account, error)
  func (m *Manager) UpdateUsage(uuid string) error
  func (m *Manager) UpdateToken(uuid string, accessToken, refreshToken string, expiresAt time.Time) error
  ```

### 验收标准
- [x] 账号可以正确保存到 JSON 文件
- [x] 可以通过 UUID 加载账号
- [x] 可以列出和删除账号

---

## Phase 3: OAuth 认证

**目标**: 实现 iFlow OAuth 登录和 Token 刷新

### 任务清单

- [x] **P3-1** 定义 OAuth 常量 (`internal/oauth/client.go`)
  ```go
  const (
      ClientID     = "10009311001"
      ClientSecret = "4Z3YjXycVsQvyGF1etiNlIBB4RsqSDtW"
      AuthURL      = "https://iflow.cn/oauth"
      TokenURL     = "https://iflow.cn/oauth/token"
      UserInfoURL  = "https://iflow.cn/api/oauth/getUserInfo"
  )
  ```

- [x] **P3-2** 实现 OAuth 客户端 (`internal/oauth/client.go`)
  ```go
  type Client struct {
      config *oauth2.Config
  }
  
  func NewClient() *Client
  func (c *Client) GetAuthURL(redirectURI, state string) string
  func (c *Client) Exchange(ctx context.Context, code string) (*Token, error)
  func (c *Client) Refresh(ctx context.Context, refreshToken string) (*Token, error)
  func (c *Client) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error)
  ```

- [x] **P3-3** 实现 Token 自动刷新器 (`internal/oauth/refresher.go`)
  ```go
  type Refresher struct {
      manager      *account.Manager
      checkInterval time.Duration
      refreshBuffer time.Duration
      stopChan      chan struct{}
  }
  
  func NewRefresher(manager *account.Manager) *Refresher
  func (r *Refresher) Start()
  func (r *Refresher) Stop()
  func (r *Refresher) shouldRefresh(account *account.Account) bool
  ```

- [x] **P3-4** 实现 OAuth 登录流程
  ```go
  func (c *Client) Login(ctx context.Context) (*Account, error)
  ```
  1. 生成授权 URL
  2. 打开浏览器
  3. 启动临时 HTTP 服务器接收回调
  4. 使用授权码换取 Token
  5. 获取用户信息 (包含 API Key)
  6. 创建账号并保存

### 验收标准
- [x] 可以生成正确的授权 URL
- [x] 可以完成 OAuth 登录流程
- [x] Token 刷新功能正常工作

---

## Phase 4: API 代理

**目标**: 实现 iFlow API 代理核心逻辑

### 任务清单

- [x] **P4-1** 实现 HMAC 签名 (`internal/proxy/signature.go`)
  ```go
  func GenerateSignature(userAgent, sessionID string, timestamp int64, apiKey string) string
  ```

- [x] **P4-2** 实现请求头构造 (`internal/proxy/headers.go`)
  ```go
  type HeaderBuilder struct {
      account *account.Account
      sessionID string
      conversationID string
  }
  
  func NewHeaderBuilder(account *account.Account) *HeaderBuilder
  func (b *HeaderBuilder) Build(stream bool) map[string]string
  ```

- [x] **P4-3** 实现模型配置 (`internal/proxy/models.go`)
  ```go
  type ModelConfig struct {
      ID          string
      Name        string
      Description string
      SupportsVision bool
  }
  
  var Models = []ModelConfig{...}
  
  func ConfigureModelParams(body map[string]interface{}, model string) map[string]interface{}
  func NormalizeResponse(response map[string]interface{}, preserveReasoning bool) map[string]interface{}
  ```

- [x] **P4-4** 实现 IFlowProxy 主类 (`internal/proxy/proxy.go`)
  ```go
  type IFlowProxy struct {
      account *account.Account
      client  *http.Client
  }
  
  func NewProxy(account *account.Account) *IFlowProxy
  func (p *IFlowProxy) ChatCompletions(ctx context.Context, req *types.ChatCompletionRequest) (*types.ChatCompletionResponse, error)
  func (p *IFlowProxy) ChatCompletionsStream(ctx context.Context, req *types.ChatCompletionRequest) (<-chan []byte, error)
  func (p *IFlowProxy) Models() []ModelConfig
  ```

- [x] **P4-5** 实现遥测上报 (`internal/proxy/telemetry.go`)
  ```go
  type Telemetry struct {
      userID string
  }
  
  func (t *Telemetry) EmitRunStarted(ctx context.Context, model, traceID string) error
  func (t *Telemetry) EmitRunError(ctx context.Context, model, traceID, errMsg string) error
  ```

### 验收标准
- [x] HMAC 签名与 Python 版本一致
- [x] 请求头包含所有必要字段
- [x] 可以正确处理流式和非流式响应
- [x] reasoning_content 处理正确

---

## Phase 5: HTTP 服务器

**目标**: 实现 HTTP 服务器和 API 端点

### 任务清单

- [x] **P5-1** 实现服务器主类 (`internal/server/server.go`)
  ```go
  type Server struct {
      config     *config.Config
      accountMgr *account.Manager
      httpServer *http.Server
  }
  
  func New(cfg *config.Config) *Server
  func (s *Server) Start() error
  func (s *Server) Stop(ctx context.Context) error
  ```

- [x] **P5-2** 实现路由 (`internal/server/routes.go`)
  ```go
  func (s *Server) setupRoutes() *http.ServeMux
  // /health
  // /v1/models
  // /v1/chat/completions
  ```

- [x] **P5-3** 实现中间件 (`internal/server/middleware.go`)
  ```go
  func LoggingMiddleware(next http.Handler) http.Handler
  func AuthMiddleware(manager *account.Manager) func(http.Handler) http.Handler
  func RequestSizeLimitMiddleware(max int64) func(http.Handler) http.Handler
  ```

- [x] **P5-4** 实现请求处理器 (`internal/server/handlers.go`)
  ```go
  func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request)
  func (s *Server) handleModels(w http.ResponseWriter, r *http.Request)
  func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request)
  ```

- [x] **P5-5** 实现 SSE 流式响应 (`internal/server/sse.go`)
  ```go
  type SSEWriter struct {
      w       http.ResponseWriter
      flusher http.Flusher
  }
  
  func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error)
  func (s *SSEWriter) WriteEvent(data string) error
  func (s *SSEWriter) WriteDone() error
  ```

### 验收标准
- [x] 服务器可以正常启动和停止
- [x] 所有 API 端点正常工作
- [x] Bearer Token 认证正常工作
- [x] SSE 流式响应正常工作

---

## Phase 6: CLI 完善

**目标**: 完善所有 CLI 命令

### 任务清单

- [x] **P6-1** 实现根命令 (`cmd/root.go`)
  ```go
  var rootCmd = &cobra.Command{
      Use:   "iflow-go",
      Short: "iFlow API 代理服务",
  }
  ```

- [x] **P6-2** 实现 serve 命令 (`cmd/serve.go`)
  ```go
  var serveCmd = &cobra.Command{
      Use:   "serve",
      Short: "启动代理服务",
      Run:   runServe,
  }
  // --host, --port, --concurrency flags
  ```

- [x] **P6-3** 实现 token 命令组 (`cmd/token.go`)
  ```go
  var tokenCmd = &cobra.Command{
      Use:   "token",
      Short: "Token 管理",
  }
  
  // token list
  // token import
  // token delete <uuid>
  // token refresh <uuid>
  ```

- [x] **P6-4** 实现 version 命令 (`cmd/version.go`)
  ```go
  var versionCmd = &cobra.Command{
      Use:   "version",
      Short: "显示版本信息",
      Run:   runVersion,
  }
  ```

- [x] **P6-5** 创建主入口 (`main.go`)
  ```go
  func main() {
      if err := rootCmd.Execute(); err != nil {
          os.Exit(1)
      }
  }
  ```

### 验收标准
- [x] 所有命令正常工作
- [x] 帮助信息清晰完整
- [x] 错误处理友好

---

## Phase 7: 测试与文档

**目标**: 完善测试和文档

### 任务清单

- [x] **P7-1** 单元测试
  - `internal/proxy/*_test.go`
  - `internal/account/*_test.go`
  - `internal/oauth/*_test.go`

- [x] **P7-2** 集成测试
  - 测试完整请求流程
  - 测试 Token 刷新流程

- [x] **P7-3** 更新 README.md
  - 项目介绍
  - 安装说明
  - 使用示例
  - 配置说明

- [x] **P7-4** 创建 API 文档
  - 端点说明
  - 请求/响应示例

### 验收标准
- [x] 测试覆盖率 > 70%
- [x] 文档完整清晰

---

## Phase 8: 官方请求对齐

**目标**: 以官方 `iflow-cli 0.5.14` 为基线，建立请求对齐专项，逐步收敛协议、遥测和传输指纹差异。

### 任务清单

- [x] **P8-1** 建立官方行为基线
  - 固定输入采集官方 CLI 的请求样本
  - 记录 `headers/body/telemetry/retry/stream` 行为
  - 定义动态字段白名单（如时间戳、trace id）

- [x] **P8-2** 对齐协议层请求构造
  - 校准 Header 条件逻辑（含 Aone 分支专有头）
  - 校准模型特定 Body 规则（含 `iFlow-ROME-30BA3B`、域名特判字段）
  - 校准 `session-id`、`conversation-id`、`traceparent` 生命周期

- [x] **P8-3** 对齐遥测事件
  - 补齐 `run_started`、`run_finished`、`run_error`
  - 统一 `cliVer`、`nodeVersion`、`osVersion` 的来源策略
  - 建立 telemetry 差异回放样本

- [x] **P8-4** 对齐传输指纹
  - 评估 Go `net/http` 与 Node/undici 差异
  - 保留 Go 发送链路，移除 Node sidecar 方案
  - 以单实现路径降低复杂度与维护风险

- [x] **P8-5** 建立回归与灰度机制
  - 增加基线回放测试与字段 diff 报告
  - 增加对齐模式灰度开关
  - 建立异常封禁回滚流程

### 验收标准
- [x] 固定样本下 Header/Body 与官方基线仅剩动态字段差异
- [x] Telemetry 事件序列与字段结构对齐
- [x] 对齐模式可按开关启停，不影响现有默认路径
- [x] 通过 `go test ./...`
- [x] 通过 `go vet ./...`

---

## 阶段完成确认

每个阶段完成后，请确认以下事项：

1. [ ] 所有任务项已完成
2. [ ] 代码通过 `go test ./...`
3. [ ] 代码通过 `go vet ./...`
4. [ ] 更新 `docs/architecture.md` (如有变更)
5. [ ] 更新 `AGENTS.md` 阶段状态追踪

---

## 归档命名规范

```
docs/archive/
└── plan-completed-{YYYYMMDD}.md
```

格式: `plan-completed-{YYYYMMDD}.md`

归档时机：所有阶段完成后

每个归档文件应包含：
1. 完成日期
2. 所有阶段的任务清单完成状态
3. 遇到的问题和解决方案
4. 验收结果
5. 备注

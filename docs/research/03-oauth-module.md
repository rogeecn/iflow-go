# OAuth 认证模块详解

**文件**: `iflow2api/oauth.py`, `iflow2api/token_refresher.py`

## 1. OAuth 配置

```python
class IFlowOAuth:
    CLIENT_ID = "10009311001"
    CLIENT_SECRET = "4Z3YjXycVsQvyGF1etiNlIBB4RsqSDtW"
    TOKEN_URL = "https://iflow.cn/oauth/token"
    USER_INFO_URL = "https://iflow.cn/api/oauth/getUserInfo"
    AUTH_URL = "https://iflow.cn/oauth"
```

---

## 2. 授权流程

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   用户      │    │  iflow2api  │    │  iFlow OAuth│    │  iFlow API  │
└──────┬──────┘    └──────┬──────┘    └──────┬──────┘    └──────┬──────┘
       │                  │                  │                  │
       │ 1. 点击登录      │                  │                  │
       │─────────────────>│                  │                  │
       │                  │ 2. 获取授权 URL  │                  │
       │                  │─────────────────>│                  │
       │                  │                  │                  │
       │ 3. 打开授权页面  │                  │                  │
       │─────────────────────────────────────>│                  │
       │                  │                  │                  │
       │ 4. 用户授权      │                  │                  │
       │─────────────────────────────────────>│                  │
       │                  │                  │                  │
       │ 5. 回调 code     │                  │                  │
       │<─────────────────────────────────────│                  │
       │                  │                  │                  │
       │ 6. 发送 code     │                  │                  │
       │─────────────────>│                  │                  │
       │                  │ 7. 换取 token    │                  │
       │                  │─────────────────>│                  │
       │                  │                  │                  │
       │                  │ 8. 获取用户信息  │                  │
       │                  │─────────────────────────────────────>│
       │                  │                  │                  │
       │                  │ 9. 返回 apiKey   │                  │
       │                  │<─────────────────────────────────────│
       │                  │                  │                  │
       │ 10. 登录成功     │                  │                  │
       │<─────────────────│                  │                  │
       │                  │                  │                  │
```

---

## 3. 核心方法

### 3.1 获取授权 URL

```python
def get_auth_url(self, redirect_uri: str, state: Optional[str] = None) -> str:
    if state is None:
        state = secrets.token_urlsafe(16)
    
    return (
        f"{self.AUTH_URL}?"
        f"client_id={self.CLIENT_ID}&"
        f"loginMethod=phone&"
        f"type=phone&"
        f"redirect={redirect_uri}&"
        f"state={state}"
    )
```

**Go 实现**:
```go
func (o *IFlowOAuth) GetAuthURL(redirectURI, state string) string {
    if state == "" {
        state = generateRandomState()
    }
    
    params := url.Values{}
    params.Set("client_id", ClientID)
    params.Set("loginMethod", "phone")
    params.Set("type", "phone")
    params.Set("redirect", redirectURI)
    params.Set("state", state)
    
    return AuthURL + "?" + params.Encode()
}
```

### 3.2 使用授权码换取 Token

```python
async def get_token(self, code: str, redirect_uri: str) -> Dict[str, Any]:
    credentials = base64.b64encode(
        f"{self.CLIENT_ID}:{self.CLIENT_SECRET}".encode()
    ).decode()
    
    response = await client.post(
        self.TOKEN_URL,
        data={
            "grant_type": "authorization_code",
            "code": code,
            "redirect_uri": redirect_uri,
            "client_id": self.CLIENT_ID,
            "client_secret": self.CLIENT_SECRET,
        },
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "Authorization": f"Basic {credentials}",
            "User-Agent": "iFlow-Cli",
        },
    )
    
    token_data = response.json()
    
    if "expires_in" in token_data:
        expires_in = token_data["expires_in"]
        token_data["expires_at"] = datetime.now() + timedelta(seconds=expires_in)
    
    return token_data
```

**Go 实现**:
```go
func (o *IFlowOAuth) GetToken(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
    data := url.Values{}
    data.Set("grant_type", "authorization_code")
    data.Set("code", code)
    data.Set("redirect_uri", redirectURI)
    data.Set("client_id", ClientID)
    data.Set("client_secret", ClientSecret)
    
    req, _ := http.NewRequestWithContext(ctx, "POST", TokenURL, 
        strings.NewReader(data.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("User-Agent", "iFlow-Cli")
    
    // Basic Auth
    credentials := base64.StdEncoding.EncodeToString(
        []byte(fmt.Sprintf("%s:%s", ClientID, ClientSecret)))
    req.Header.Set("Authorization", "Basic "+credentials)
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var result TokenResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    if result.ExpiresIn > 0 {
        result.ExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
    }
    
    return &result, nil
}
```

### 3.3 刷新 Token

```python
async def refresh_token(self, refresh_token: str) -> Dict[str, Any]:
    response = await client.post(
        self.TOKEN_URL,
        data={
            "grant_type": "refresh_token",
            "client_id": self.CLIENT_ID,
            "client_secret": self.CLIENT_SECRET,
            "refresh_token": refresh_token,
        },
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "Authorization": f"Basic {credentials}",
            "User-Agent": "iFlow-Cli",
        },
    )
    
    if response.status_code == 400:
        error_data = response.json()
        if "invalid_grant" in error_data.get("error", ""):
            raise ValueError("refresh_token 无效或已过期")
    
    token_data = response.json()
    
    # 检查 iFlow 特有格式：HTTP 200 但 success=false
    if token_data.get("success") is False:
        error_msg = token_data.get("message", "未知错误")
        if "太多" in error_msg:
            raise ValueError(f"服务器过载: {error_msg}")
        else:
            raise ValueError(f"OAuth 刷新失败: {error_msg}")
    
    return token_data
```

### 3.4 获取用户信息

```python
async def get_user_info(self, access_token: str) -> Dict[str, Any]:
    # accessToken 作为 URL 查询参数
    response = await client.get(
        f"{self.USER_INFO_URL}?accessToken={access_token}",
        headers={
            "Accept": "application/json",
            "User-Agent": "iFlow-Cli",
        },
    )
    
    result = response.json()
    
    if result.get("success") and result.get("data"):
        return result["data"]  # 包含 apiKey
    else:
        raise ValueError("获取用户信息失败")
```

---

## 4. Token 自动刷新

**文件**: `iflow2api/token_refresher.py`

### 4.1 刷新策略

```python
# 配置常量
CHECK_INTERVAL_SECONDS = 6 * 60 * 60    # 每6小时检查一次
REFRESH_BUFFER_SECONDS = 24 * 60 * 60   # 提前24小时刷新（与 iflow-cli 一致）
RETRY_COUNT = 5                          # 重试次数
RETRY_DELAY_SECONDS = 30                 # 重试间隔
RETRY_EXPONENTIAL_BACKOFF = True         # 指数退避
```

### 4.2 刷新器类

```python
class OAuthTokenRefresher:
    def __init__(self):
        self._running = False
        self._thread: Optional[threading.Thread] = None
        self._stop_event = threading.Event()
        self._on_refresh_callback: Optional[Callable] = None
        self._loop: Optional[asyncio.AbstractEventLoop] = None
    
    def _should_refresh(self, config: IFlowConfig) -> bool:
        """
        刷新条件：
        1. 有 refresh_token
        2. 有过期时间
        3. 距离过期时间小于24小时
        """
        if not config.oauth_refresh_token:
            return False
        
        expires_at = config.api_key_expires_at or config.oauth_expires_at
        if not expires_at:
            return False
        
        time_until_expiry = expires_at - datetime.now()
        
        # 已过期或即将过期
        return time_until_expiry.total_seconds() <= REFRESH_BUFFER_SECONDS
    
    async def _refresh_token_with_retry(self, config: IFlowConfig) -> bool:
        """带重试机制的刷新"""
        oauth = IFlowOAuth()
        
        for attempt in range(1, self.retry_count + 1):
            try:
                token_data = await oauth.refresh_token(config.oauth_refresh_token)
                
                # 更新配置
                config.oauth_access_token = token_data.get("access_token", "")
                if token_data.get("refresh_token"):
                    config.oauth_refresh_token = token_data["refresh_token"]
                if token_data.get("expires_at"):
                    config.oauth_expires_at = token_data["expires_at"]
                    config.api_key_expires_at = token_data["expires_at"]
                
                save_iflow_config(config)
                
                if self._on_refresh_callback:
                    self._on_refresh_callback(token_data)
                
                return True
                
            except Exception as e:
                error_msg = str(e)
                
                # 临时错误（服务器过载）- 重试
                if any(x in error_msg for x in ["太多", "过载", "timeout", "503", "502", "429"]):
                    delay = min(self.retry_delay * (2 ** (attempt - 1)), 300)
                    await asyncio.sleep(delay)
                    continue
                
                # 凭证无效 - 停止重试
                if any(x in error_msg for x in ["invalid_grant", "已过期", "无效"]):
                    break
        
        return False
```

### 4.3 Go 实现

```go
type OAuthTokenRefresher struct {
    checkInterval   time.Duration
    refreshBuffer   time.Duration
    retryCount      int
    retryDelay      time.Duration
    stopChan        chan struct{}
    onRefresh       func(token *TokenResponse)
}

func NewOAuthTokenRefresher() *OAuthTokenRefresher {
    return &OAuthTokenRefresher{
        checkInterval: 6 * time.Hour,
        refreshBuffer: 24 * time.Hour,
        retryCount:    5,
        retryDelay:    30 * time.Second,
        stopChan:      make(chan struct{}),
    }
}

func (r *OAuthTokenRefresher) Start() {
    ticker := time.NewTicker(r.checkInterval)
    go func() {
        for {
            select {
            case <-ticker.C:
                r.checkAndRefresh()
            case <-r.stopChan:
                ticker.Stop()
                return
            }
        }
    }()
}

func (r *OAuthTokenRefresher) Stop() {
    close(r.stopChan)
}

func (r *OAuthTokenRefresher) shouldRefresh(config *IFlowConfig) bool {
    if config.OAuthRefreshToken == "" {
        return false
    }
    
    expiresAt := config.APIKeyExpiresAt
    if expiresAt.IsZero() {
        return false
    }
    
    return time.Until(expiresAt) <= r.refreshBuffer
}

func (r *OAuthTokenRefresher) refreshTokenWithRetry(config *IFlowConfig) bool {
    oauth := NewIFlowOAuth()
    
    for attempt := 1; attempt <= r.retryCount; attempt++ {
        token, err := oauth.RefreshToken(config.OAuthRefreshToken)
        if err == nil {
            // 更新配置
            config.OAuthAccessToken = token.AccessToken
            if token.RefreshToken != "" {
                config.OAuthRefreshToken = token.RefreshToken
            }
            config.OAuthExpiresAt = token.ExpiresAt
            config.APIKeyExpiresAt = token.ExpiresAt
            
            // 保存配置
            SaveIFlowConfig(config)
            
            if r.onRefresh != nil {
                r.onRefresh(token)
            }
            
            return true
        }
        
        // 判断错误类型
        if isPermanentError(err) {
            break
        }
        
        // 指数退避
        delay := r.retryDelay * time.Duration(1<<(attempt-1))
        if delay > 5*time.Minute {
            delay = 5 * time.Minute
        }
        time.Sleep(delay)
    }
    
    return false
}
```

---

## 5. 数据结构

### 5.1 Token 响应

```go
type TokenResponse struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenType    string    `json:"token_type"`
    ExpiresIn    int       `json:"expires_in"`
    ExpiresAt    time.Time `json:"-"`
    Scope        string    `json:"scope"`
}
```

### 5.2 用户信息响应

```go
type UserInfo struct {
    UserID    string `json:"userId"`
    Username  string `json:"username"`
    Phone     string `json:"phone"`
    Email     string `json:"email"`
    APIKey    string `json:"apiKey"`
    Nickname  string `json:"nickname"`
    Avatar    string `json:"avatar"`
}
```
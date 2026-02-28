package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
	"golang.org/x/oauth2"
)

const (
	ClientID     = "10009311001"
	ClientSecret = "4Z3YjXycVsQvyGF1etiNlIBB4RsqSDtW"
	AuthURL      = "https://iflow.cn/oauth"
	TokenURL     = "https://iflow.cn/oauth/token"
	UserInfoURL  = "https://iflow.cn/api/oauth/getUserInfo"
)

const (
	defaultCallbackHost = "127.0.0.1"
	defaultCallbackPath = "/oauth2callback"
	defaultStartPort    = 11451
	maxPortAttempts     = 50
	defaultLoginTimeout = 60 * time.Second
	defaultDataDir      = "./data"
	defaultBaseURL      = "https://apis.iflow.cn/v1"
)

type callbackPayload struct {
	Code  string
	Error string
	State string
}

type callbackServerRunner interface {
	URL() string
	Wait(ctx context.Context, timeout time.Duration) (callbackPayload, error)
	Close(ctx context.Context) error
}

type Client struct {
	config     *oauth2.Config
	httpClient *http.Client
	manager    *account.Manager

	authURL     string
	tokenURL    string
	userInfoURL string

	browserOpener         func(ctx context.Context, targetURL string) error
	callbackServerFactory func(startPort, attempts int) (callbackServerRunner, error)
	stateGenerator        func(size int) string
	loginTimeout          time.Duration
}

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type UserInfo struct {
	APIKey   string `json:"apiKey"`
	Username string `json:"username"`
	Phone    string `json:"phone"`
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int64  `json:"expires_in"`
	Error        string `json:"error"`
	Success      *bool  `json:"success"`
	Message      string `json:"message"`
	Code         string `json:"code"`
}

type userInfoResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Data    UserInfo `json:"data"`
}

func NewClient() *Client {
	dataDir := strings.TrimSpace(os.Getenv("IFLOW_DATA_DIR"))
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	return NewClientWithManager(account.NewManager(dataDir))
}

func NewClientWithManager(manager *account.Manager) *Client {
	if manager == nil {
		manager = account.NewManager(defaultDataDir)
	}

	return &Client{
		config: &oauth2.Config{
			ClientID:     ClientID,
			ClientSecret: ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  AuthURL,
				TokenURL: TokenURL,
			},
		},
		httpClient:            &http.Client{Timeout: 30 * time.Second},
		manager:               manager,
		authURL:               AuthURL,
		tokenURL:              TokenURL,
		userInfoURL:           UserInfoURL,
		browserOpener:         openBrowser,
		callbackServerFactory: newCallbackServerRunner,
		stateGenerator:        generateRandomState,
		loginTimeout:          defaultLoginTimeout,
	}
}

func (c *Client) GetAuthURL(redirectURI, state string) string {
	if strings.TrimSpace(state) == "" {
		state = generateRandomState(16)
	}

	params := url.Values{}
	params.Set("client_id", ClientID)
	params.Set("loginMethod", "phone")
	params.Set("type", "phone")
	params.Set("redirect", redirectURI)
	params.Set("state", state)

	return c.authURL + "?" + params.Encode()
}

func (c *Client) Exchange(ctx context.Context, code string) (*Token, error) {
	return c.exchangeWithRedirect(ctx, code, fmt.Sprintf("http://%s:%d%s", defaultCallbackHost, defaultStartPort, defaultCallbackPath))
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (*Token, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token: empty refresh token")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", ClientID)
	form.Set("client_secret", ClientSecret)
	form.Set("refresh_token", refreshToken)

	return c.requestToken(ctx, form, true)
}

func (c *Client) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("get user info: empty access token")
	}

	reqURL := fmt.Sprintf("%s?accessToken=%s", c.userInfoURL, url.QueryEscape(accessToken))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("get user info: create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "iFlow-Cli")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get user info: send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("get user info: read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("get user info: access token invalid or expired")
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("get user info: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload userInfoResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("get user info: parse json: %w", err)
	}

	if !payload.Success || strings.TrimSpace(payload.Data.APIKey) == "" {
		msg := strings.TrimSpace(payload.Message)
		if msg == "" {
			msg = "missing api key from oauth user info"
		}
		return nil, fmt.Errorf("get user info: %s", msg)
	}

	return &payload.Data, nil
}

func (c *Client) Login(ctx context.Context) (*account.Account, error) {
	cbServer, err := c.callbackServerFactory(defaultStartPort, maxPortAttempts)
	if err != nil {
		return nil, fmt.Errorf("oauth login: create callback server: %w", err)
	}
	defer cbServer.Close(context.Background())

	state := c.stateGenerator(16)
	authURL := c.GetAuthURL(cbServer.URL(), state)
	if err := c.browserOpener(ctx, authURL); err != nil {
		return nil, fmt.Errorf("oauth login: open browser: %w", err)
	}

	result, err := cbServer.Wait(ctx, c.loginTimeout)
	if err != nil {
		return nil, fmt.Errorf("oauth login: wait callback: %w", err)
	}
	if strings.TrimSpace(result.Error) != "" {
		return nil, fmt.Errorf("oauth login: authorization failed: %s", result.Error)
	}
	if result.State != state {
		return nil, fmt.Errorf("oauth login: invalid state")
	}
	if strings.TrimSpace(result.Code) == "" {
		return nil, fmt.Errorf("oauth login: missing authorization code")
	}

	token, err := c.exchangeWithRedirect(ctx, result.Code, cbServer.URL())
	if err != nil {
		return nil, fmt.Errorf("oauth login: exchange token: %w", err)
	}

	user, err := c.GetUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("oauth login: get user info: %w", err)
	}

	acct, err := c.manager.Create(user.APIKey, defaultBaseURL)
	if err != nil {
		return nil, fmt.Errorf("oauth login: create account: %w", err)
	}

	if err := c.manager.UpdateToken(acct.UUID, token.AccessToken, token.RefreshToken, token.ExpiresAt); err != nil {
		return nil, fmt.Errorf("oauth login: save token: %w", err)
	}

	stored, err := c.manager.Get(acct.UUID)
	if err != nil {
		return nil, fmt.Errorf("oauth login: reload account: %w", err)
	}

	return stored, nil
}

func (c *Client) exchangeWithRedirect(ctx context.Context, code, redirectURI string) (*Token, error) {
	code = strings.TrimSpace(code)
	redirectURI = strings.TrimSpace(redirectURI)
	if code == "" {
		return nil, fmt.Errorf("exchange token: empty code")
	}
	if redirectURI == "" {
		return nil, fmt.Errorf("exchange token: empty redirect uri")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", ClientID)
	form.Set("client_secret", ClientSecret)

	return c.requestToken(ctx, form, false)
}

func (c *Client) requestToken(ctx context.Context, form url.Values, refresh bool) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("request token: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "iFlow-Cli")
	req.Header.Set("Authorization", "Basic "+basicCredentials(ClientID, ClientSecret))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request token: send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("request token: read response: %w", err)
	}

	var payload oauthTokenResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("request token: parse json: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest && refresh && strings.Contains(strings.ToLower(payload.Error), "invalid_grant") {
		return nil, fmt.Errorf("request token: refresh token invalid or expired")
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("request token: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if payload.Success != nil && !*payload.Success {
		msg := strings.TrimSpace(payload.Message)
		if msg == "" {
			msg = "oauth response indicates failure"
		}
		return nil, fmt.Errorf("request token: %s", msg)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return nil, fmt.Errorf("request token: missing access_token")
	}

	token := &Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		TokenType:    payload.TokenType,
		Scope:        payload.Scope,
		ExpiresIn:    payload.ExpiresIn,
	}
	if payload.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().UTC().Add(time.Duration(payload.ExpiresIn) * time.Second)
	}

	return token, nil
}

func basicCredentials(clientID, clientSecret string) string {
	return base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
}

func generateRandomState(size int) string {
	if size <= 0 {
		size = 16
	}

	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("state-%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

type callbackServer struct {
	server   *http.Server
	listener net.Listener
	resultCh chan callbackPayload
	once     sync.Once
}

func newCallbackServerRunner(startPort, attempts int) (callbackServerRunner, error) {
	return newCallbackServer(startPort, attempts)
}

func newCallbackServer(startPort, attempts int) (*callbackServer, error) {
	if attempts <= 0 {
		attempts = 1
	}

	var listener net.Listener
	var err error
	port := startPort
	for i := 0; i < attempts; i++ {
		listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", defaultCallbackHost, port+i))
		if err == nil {
			break
		}
	}
	if listener == nil {
		return nil, fmt.Errorf("no available callback port from %d (%d attempts)", startPort, attempts)
	}

	cb := &callbackServer{
		listener: listener,
		resultCh: make(chan callbackPayload, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(defaultCallbackPath, cb.handleCallback)

	cb.server = &http.Server{Handler: mux}

	go func() {
		_ = cb.server.Serve(listener)
	}()

	return cb, nil
}

func (c *callbackServer) URL() string {
	return "http://" + c.listener.Addr().String() + defaultCallbackPath
}

func (c *callbackServer) Wait(ctx context.Context, timeout time.Duration) (callbackPayload, error) {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case payload := <-c.resultCh:
		return payload, nil
	case <-timer.C:
		return callbackPayload{}, fmt.Errorf("callback timeout")
	case <-ctx.Done():
		return callbackPayload{}, ctx.Err()
	}
}

func (c *callbackServer) Close(ctx context.Context) error {
	var closeErr error
	c.once.Do(func() {
		closeErr = c.server.Shutdown(ctx)
	})
	return closeErr
}

func (c *callbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	payload := callbackPayload{
		Code:  q.Get("code"),
		Error: q.Get("error"),
		State: q.Get("state"),
	}

	select {
	case c.resultCh <- payload:
	default:
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if strings.TrimSpace(payload.Code) != "" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body><h1>OAuth Success</h1><p>You can close this page.</p></body></html>"))
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte("<html><body><h1>OAuth Failed</h1><p>Please return to CLI.</p></body></html>"))
}

func openBrowser(ctx context.Context, targetURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", targetURL)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", targetURL)
	}
	return cmd.Start()
}

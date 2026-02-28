package oauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newJSONResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type stubCallbackServer struct {
	url     string
	payload callbackPayload
	err     error
	closed  bool
}

func (s *stubCallbackServer) URL() string {
	return s.url
}

func (s *stubCallbackServer) Wait(_ context.Context, _ time.Duration) (callbackPayload, error) {
	return s.payload, s.err
}

func (s *stubCallbackServer) Close(_ context.Context) error {
	s.closed = true
	return nil
}

func TestGetAuthURL(t *testing.T) {
	client := NewClient()
	client.authURL = "https://example.com/oauth"

	got := client.GetAuthURL("http://127.0.0.1:11451/oauth2callback", "test-state")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}

	q := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "example.com" {
		t.Fatalf("unexpected auth url host: %s", got)
	}
	if q.Get("client_id") != ClientID {
		t.Fatalf("client_id = %q, want %q", q.Get("client_id"), ClientID)
	}
	if q.Get("redirect") != "http://127.0.0.1:11451/oauth2callback" {
		t.Fatalf("redirect = %q", q.Get("redirect"))
	}
	if q.Get("state") != "test-state" {
		t.Fatalf("state = %q, want %q", q.Get("state"), "test-state")
	}
}

func TestNewClientWithManagerNil(t *testing.T) {
	client := NewClientWithManager(nil)
	if client == nil || client.manager == nil {
		t.Fatal("NewClientWithManager(nil) should initialize defaults")
	}
	if client.browserOpener == nil || client.callbackServerFactory == nil || client.stateGenerator == nil {
		t.Fatal("NewClientWithManager(nil) should initialize injectable helpers")
	}
}

func TestExchange(t *testing.T) {
	client := NewClient()
	client.tokenURL = "https://example.com/oauth/token"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "Basic "+basicCredentials(ClientID, ClientSecret) {
				t.Fatalf("Authorization = %q", got)
			}
			if got := r.Header.Get("User-Agent"); got != "iFlow-Cli" {
				t.Fatalf("User-Agent = %q", got)
			}

			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm error: %v", err)
			}
			if r.Form.Get("grant_type") != "authorization_code" {
				t.Fatalf("grant_type = %q", r.Form.Get("grant_type"))
			}
			if r.Form.Get("code") != "auth-code" {
				t.Fatalf("code = %q", r.Form.Get("code"))
			}
			if r.Form.Get("redirect_uri") != "http://127.0.0.1:18080/oauth2callback" {
				t.Fatalf("redirect_uri = %q", r.Form.Get("redirect_uri"))
			}

			return newJSONResponse(http.StatusOK, `{"access_token":"access-1","refresh_token":"refresh-1","expires_in":3600}`), nil
		}),
	}

	token, err := client.exchangeWithRedirect(context.Background(), "auth-code", "http://127.0.0.1:18080/oauth2callback")
	if err != nil {
		t.Fatalf("exchangeWithRedirect error: %v", err)
	}
	if token.AccessToken != "access-1" {
		t.Fatalf("access token = %q", token.AccessToken)
	}
	if token.RefreshToken != "refresh-1" {
		t.Fatalf("refresh token = %q", token.RefreshToken)
	}
	if token.ExpiresIn != 3600 {
		t.Fatalf("expires_in = %d", token.ExpiresIn)
	}
	if token.ExpiresAt.IsZero() {
		t.Fatal("expires_at is zero")
	}
}

func TestRefreshInvalidGrant(t *testing.T) {
	client := NewClient()
	client.tokenURL = "https://example.com/oauth/token"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return newJSONResponse(http.StatusBadRequest, `{"error":"invalid_grant"}`), nil
		}),
	}

	_, err := client.Refresh(context.Background(), "refresh-token")
	if err == nil || !strings.Contains(err.Error(), "refresh token invalid or expired") {
		t.Fatalf("Refresh error = %v, want invalid_grant error", err)
	}
}

func TestRefreshSuccessFalse(t *testing.T) {
	client := NewClient()
	client.tokenURL = "https://example.com/oauth/token"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return newJSONResponse(http.StatusOK, `{"success":false,"message":"服务器请求太多"}`), nil
		}),
	}

	_, err := client.Refresh(context.Background(), "refresh-token")
	if err == nil || !strings.Contains(err.Error(), "服务器请求太多") {
		t.Fatalf("Refresh error = %v, want success=false error", err)
	}
}

func TestGetUserInfo(t *testing.T) {
	client := NewClient()
	client.userInfoURL = "https://example.com/api/oauth/getUserInfo"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("User-Agent"); got != "iFlow-Cli" {
				t.Fatalf("User-Agent = %q", got)
			}
			if got := r.URL.Query().Get("accessToken"); got != "token-123" {
				t.Fatalf("accessToken = %q", got)
			}
			return newJSONResponse(http.StatusOK, `{"success":true,"data":{"apiKey":"sk-abc","username":"tester"}}`), nil
		}),
	}

	user, err := client.GetUserInfo(context.Background(), "token-123")
	if err != nil {
		t.Fatalf("GetUserInfo error: %v", err)
	}
	if user.APIKey != "sk-abc" {
		t.Fatalf("api key = %q", user.APIKey)
	}
	if user.Username != "tester" {
		t.Fatalf("username = %q", user.Username)
	}
}

func TestLogin(t *testing.T) {
	manager := account.NewManager(t.TempDir())
	client := NewClientWithManager(manager)
	client.tokenURL = "https://example.com/oauth/token"
	client.userInfoURL = "https://example.com/api/oauth/getUserInfo"

	callback := &stubCallbackServer{
		url: "http://127.0.0.1:18080/oauth2callback",
		payload: callbackPayload{
			Code:  "auth-code",
			State: "fixed-state",
		},
	}

	var openedURL string
	client.callbackServerFactory = func(startPort, attempts int) (callbackServerRunner, error) {
		if startPort != defaultStartPort || attempts != maxPortAttempts {
			t.Fatalf("unexpected callback args: port=%d attempts=%d", startPort, attempts)
		}
		return callback, nil
	}
	client.stateGenerator = func(int) string { return "fixed-state" }
	client.browserOpener = func(_ context.Context, targetURL string) error {
		openedURL = targetURL
		return nil
	}
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.String() {
			case client.tokenURL:
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm error: %v", err)
				}
				if r.Form.Get("code") != "auth-code" {
					t.Fatalf("code = %q", r.Form.Get("code"))
				}
				if r.Form.Get("redirect_uri") != callback.url {
					t.Fatalf("redirect_uri = %q", r.Form.Get("redirect_uri"))
				}
				return newJSONResponse(http.StatusOK, `{"access_token":"oauth-access","refresh_token":"oauth-refresh","expires_in":3600}`), nil
			case client.userInfoURL + "?accessToken=oauth-access":
				return newJSONResponse(http.StatusOK, `{"success":true,"data":{"apiKey":"sk-login","username":"u1"}}`), nil
			default:
				t.Fatalf("unexpected request url: %s", r.URL.String())
				return nil, nil
			}
		}),
	}

	acct, err := client.Login(context.Background())
	if err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if !callback.closed {
		t.Fatal("callback server was not closed")
	}
	if acct.UUID == "" {
		t.Fatal("uuid is empty")
	}
	if acct.APIKey != "sk-login" {
		t.Fatalf("api key = %q", acct.APIKey)
	}
	if acct.OAuthAccessToken != "oauth-access" {
		t.Fatalf("access token = %q", acct.OAuthAccessToken)
	}
	if acct.OAuthRefreshToken != "oauth-refresh" {
		t.Fatalf("refresh token = %q", acct.OAuthRefreshToken)
	}
	if acct.OAuthExpiresAt.IsZero() {
		t.Fatal("expires at is zero")
	}
	if !strings.Contains(openedURL, "state=fixed-state") {
		t.Fatalf("auth url missing state: %s", openedURL)
	}
	if !strings.Contains(openedURL, url.QueryEscape(callback.url)) {
		t.Fatalf("auth url missing redirect: %s", openedURL)
	}
}

func TestBasicCredentials(t *testing.T) {
	got := basicCredentials("a", "b")
	want := base64.StdEncoding.EncodeToString([]byte("a:b"))
	if got != want {
		t.Fatalf("basicCredentials = %q, want %q", got, want)
	}
}

func TestGenerateRandomState(t *testing.T) {
	got := generateRandomState(8)
	if got == "" {
		t.Fatal("generateRandomState returned empty string")
	}
}

func TestExchangeEmptyCode(t *testing.T) {
	client := NewClient()
	_, err := client.exchangeWithRedirect(context.Background(), "", "http://127.0.0.1:18080/oauth2callback")
	if err == nil || !strings.Contains(err.Error(), "empty code") {
		t.Fatalf("expected empty code error, got: %v", err)
	}
}

func TestRefreshEmptyToken(t *testing.T) {
	client := NewClient()
	_, err := client.Refresh(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "empty refresh token") {
		t.Fatalf("expected empty refresh token error, got: %v", err)
	}
}

func TestGetUserInfoEmptyToken(t *testing.T) {
	client := NewClient()
	_, err := client.GetUserInfo(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "empty access token") {
		t.Fatalf("expected empty access token error, got: %v", err)
	}
}

func TestRequestTokenInvalidJSON(t *testing.T) {
	client := NewClient()
	client.tokenURL = "https://example.com/oauth/token"
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return newJSONResponse(http.StatusOK, `not-json`), nil
		}),
	}

	_, err := client.Refresh(context.Background(), "refresh")
	if err == nil || !strings.Contains(err.Error(), "parse json") {
		t.Fatalf("expected parse json error, got: %v", err)
	}
}

func TestLoginOpenBrowserError(t *testing.T) {
	manager := account.NewManager(t.TempDir())
	client := NewClientWithManager(manager)
	client.callbackServerFactory = func(int, int) (callbackServerRunner, error) {
		return &stubCallbackServer{
			url: "http://127.0.0.1:18080/oauth2callback",
		}, nil
	}
	client.stateGenerator = func(int) string { return "state" }
	client.browserOpener = func(context.Context, string) error {
		return fmt.Errorf("open browser failed")
	}

	_, err := client.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "open browser") {
		t.Fatalf("expected open browser error, got: %v", err)
	}
}

func TestLoginAuthorizationFailed(t *testing.T) {
	manager := account.NewManager(t.TempDir())
	client := NewClientWithManager(manager)
	client.callbackServerFactory = func(int, int) (callbackServerRunner, error) {
		return &stubCallbackServer{
			url: "http://127.0.0.1:18080/oauth2callback",
			payload: callbackPayload{
				Error: "access_denied",
				State: "state",
			},
		}, nil
	}
	client.stateGenerator = func(int) string { return "state" }
	client.browserOpener = func(context.Context, string) error { return nil }

	_, err := client.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "authorization failed") {
		t.Fatalf("expected authorization failed error, got: %v", err)
	}
}

func TestLoginInvalidState(t *testing.T) {
	manager := account.NewManager(t.TempDir())
	client := NewClientWithManager(manager)
	client.callbackServerFactory = func(int, int) (callbackServerRunner, error) {
		return &stubCallbackServer{
			url: "http://127.0.0.1:18080/oauth2callback",
			payload: callbackPayload{
				Code:  "auth-code",
				State: "different-state",
			},
		}, nil
	}
	client.stateGenerator = func(int) string { return "state" }
	client.browserOpener = func(context.Context, string) error { return nil }

	_, err := client.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid state") {
		t.Fatalf("expected invalid state error, got: %v", err)
	}
}

func TestLoginMissingAuthorizationCode(t *testing.T) {
	manager := account.NewManager(t.TempDir())
	client := NewClientWithManager(manager)
	client.callbackServerFactory = func(int, int) (callbackServerRunner, error) {
		return &stubCallbackServer{
			url: "http://127.0.0.1:18080/oauth2callback",
			payload: callbackPayload{
				State: "state",
			},
		}, nil
	}
	client.stateGenerator = func(int) string { return "state" }
	client.browserOpener = func(context.Context, string) error { return nil }

	_, err := client.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "missing authorization code") {
		t.Fatalf("expected missing authorization code error, got: %v", err)
	}
}

func TestCallbackServerWaitTimeout(t *testing.T) {
	cb := &callbackServer{
		resultCh: make(chan callbackPayload, 1),
	}
	_, err := cb.Wait(context.Background(), 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "callback timeout") {
		t.Fatalf("expected callback timeout error, got: %v", err)
	}
}

func TestCallbackServerWaitContextCanceled(t *testing.T) {
	cb := &callbackServer{
		resultCh: make(chan callbackPayload, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := cb.Wait(ctx, time.Second)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled error, got: %v", err)
	}
}

func TestCallbackServerHandleCallbackSuccess(t *testing.T) {
	cb := &callbackServer{
		resultCh: make(chan callbackPayload, 1),
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth2callback?code=c1&state=s1", nil)
	rec := httptest.NewRecorder()
	cb.handleCallback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	select {
	case payload := <-cb.resultCh:
		if payload.Code != "c1" || payload.State != "s1" {
			t.Fatalf("unexpected payload: %+v", payload)
		}
	default:
		t.Fatal("expected callback payload in channel")
	}
}

func TestCallbackServerHandleCallbackFailure(t *testing.T) {
	cb := &callbackServer{
		resultCh: make(chan callbackPayload, 1),
	}

	req := httptest.NewRequest(http.MethodGet, "/oauth2callback?error=access_denied&state=s1", nil)
	rec := httptest.NewRecorder()
	cb.handleCallback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	select {
	case payload := <-cb.resultCh:
		if payload.Error != "access_denied" || payload.State != "s1" {
			t.Fatalf("unexpected payload: %+v", payload)
		}
	default:
		t.Fatal("expected callback payload in channel")
	}
}

func TestCallbackServerClose(t *testing.T) {
	cb := &callbackServer{
		server: &http.Server{},
	}
	_ = cb.Close(context.Background())
}

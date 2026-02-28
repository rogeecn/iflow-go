package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
)

const (
	IFLOWCLIUserAgent   = "iFlow-Cli"
	IFLOWCLIVersion     = "0.5.14"
	aoneClientTypeValue = "iflow-cli"
)

type HeaderBuilder struct {
	account        *account.Account
	sessionID      string
	conversationID string

	now                  func() time.Time
	traceparentGenerator func() string
}

func NewHeaderBuilder(acct *account.Account) *HeaderBuilder {
	return &HeaderBuilder{
		account:              acct,
		sessionID:            "session-" + account.GenerateUUID(),
		conversationID:       account.GenerateUUID(),
		now:                  time.Now,
		traceparentGenerator: generateTraceparent,
	}
}

func (b *HeaderBuilder) Build(stream bool, traceparent string) map[string]string {
	_ = stream
	b.ensureIDs()

	headers := map[string]string{
		"Content-Type":    "application/json",
		"user-agent":      IFLOWCLIUserAgent,
		"session-id":      b.sessionID,
		"conversation-id": b.conversationID,
	}
	if strings.TrimSpace(traceparent) != "" {
		headers["traceparent"] = traceparent
	}
	if b.isAoneEndpoint() {
		headers["X-Client-Type"] = aoneClientTypeValue
		headers["X-Client-Version"] = IFLOWCLIVersion
	}

	apiKey := b.apiKey()
	if apiKey == "" {
		return headers
	}

	headers["Authorization"] = "Bearer " + apiKey
	timestamp := b.now().UnixMilli()
	signature := GenerateSignature(IFLOWCLIUserAgent, b.sessionID, timestamp, apiKey)
	if signature != "" {
		headers["x-iflow-signature"] = signature
		headers["x-iflow-timestamp"] = strconv.FormatInt(timestamp, 10)
	}

	return headers
}

func (b *HeaderBuilder) apiKey() string {
	if b == nil || b.account == nil {
		return ""
	}
	return strings.TrimSpace(b.account.APIKey)
}

func (b *HeaderBuilder) ensureIDs() {
	if b == nil {
		return
	}
	if strings.TrimSpace(b.sessionID) == "" {
		b.sessionID = "session-" + account.GenerateUUID()
	}
	if strings.TrimSpace(b.conversationID) == "" {
		b.conversationID = account.GenerateUUID()
	}
}

func (b *HeaderBuilder) isAoneEndpoint() bool {
	if b == nil || b.account == nil {
		return false
	}
	return isAoneEndpoint(b.account.BaseURL)
}

func generateTraceparent() string {
	return fmt.Sprintf("00-%s-%s-01", randomHex(16), randomHex(8))
}

func randomHex(bytesN int) string {
	if bytesN <= 0 {
		return ""
	}

	buf := make([]byte, bytesN)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", bytesN*2)
	}
	return hex.EncodeToString(buf)
}

func isAoneEndpoint(baseURL string) bool {
	host := normalizedHost(baseURL)
	if host == "" {
		return false
	}

	return strings.Contains(host, "alibaba-inc.com") || strings.Contains(host, "aone")
}

func normalizedHost(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Host != "" {
		return strings.ToLower(parsed.Hostname())
	}

	return strings.ToLower(strings.TrimSpace(trimmed))
}

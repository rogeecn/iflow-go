package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rogeecn/iflow-go/internal/account"
)

const (
	IFLOWCLIUserAgent = "iFlow-Cli"
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

func (b *HeaderBuilder) Build(stream bool) map[string]string {
	_ = stream
	b.ensureIDs()

	headers := map[string]string{
		"Content-Type":    "application/json",
		"user-agent":      IFLOWCLIUserAgent,
		"session-id":      b.sessionID,
		"conversation-id": b.conversationID,
		"accept":          "*/*",
		"accept-language": "*",
		"sec-fetch-mode":  "cors",
		"accept-encoding": "br, gzip, deflate",
		"traceparent":     b.traceparentGenerator(),
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

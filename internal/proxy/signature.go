package proxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// GenerateSignature builds iFlow HMAC-SHA256 signature:
// message = "{userAgent}:{sessionID}:{timestamp}", key = apiKey.
func GenerateSignature(userAgent, sessionID string, timestamp int64, apiKey string) string {
	if strings.TrimSpace(apiKey) == "" {
		return ""
	}

	message := fmt.Sprintf("%s:%s:%d", userAgent, sessionID, timestamp)
	mac := hmac.New(sha256.New, []byte(apiKey))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

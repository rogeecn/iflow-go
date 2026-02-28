package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rogeecn/iflow-go/pkg/types"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}

	acct, ok := accountFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "missing account context", "invalid_request_error", "invalid_api_key")
		return
	}

	models := s.newProxy(acct).Models()
	now := time.Now().Unix()
	data := make([]map[string]interface{}, 0, len(models))
	for _, m := range models {
		data = append(data, map[string]interface{}{
			"id":         m.ID,
			"object":     "model",
			"created":    now,
			"owned_by":   "iflow",
			"permission": []interface{}{},
			"root":       m.ID,
			"parent":     nil,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"object": "list",
		"data":   data,
	})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}

	acct, ok := accountFromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "missing account context", "invalid_request_error", "invalid_api_key")
		return
	}

	var reqBody types.ChatCompletionRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqBody); err != nil {
		if isBodyTooLarge(err) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "request body too large", "invalid_request_error", "request_too_large")
			return
		}
		writeAPIError(w, http.StatusBadRequest, "invalid request body", "invalid_request_error", "bad_request")
		return
	}
	if reqBody.Model == "" || len(reqBody.Messages) == 0 {
		writeAPIError(w, http.StatusBadRequest, "model and messages are required", "invalid_request_error", "bad_request")
		return
	}

	client := s.newProxy(acct)
	if reqBody.Stream {
		s.handleStreamChatCompletions(r.Context(), w, client, &reqBody, acct.UUID)
		return
	}

	resp, err := client.ChatCompletions(r.Context(), &reqBody)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, fmt.Sprintf("upstream request failed: %v", err), "api_error", "upstream_error")
		return
	}

	_ = s.accountMgr.UpdateUsage(acct.UUID)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStreamChatCompletions(ctx context.Context, w http.ResponseWriter, client proxyClient, reqBody *types.ChatCompletionRequest, uuid string) {
	stream, err := client.ChatCompletionsStream(ctx, reqBody)
	if err != nil {
		writeAPIError(w, http.StatusBadGateway, fmt.Sprintf("upstream stream failed: %v", err), "api_error", "upstream_error")
		return
	}

	sse, err := NewSSEWriter(w)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(), "internal_error", "internal_error")
		return
	}

	doneWritten := false
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-stream:
			if !ok {
				if !doneWritten {
					_ = sse.WriteDone()
				}
				_ = s.accountMgr.UpdateUsage(uuid)
				return
			}

			wroteDone, writeErr := writeProxyChunkAsSSE(sse, chunk)
			if writeErr != nil {
				return
			}
			if wroteDone {
				doneWritten = true
			}
		}
	}
}

func writeProxyChunkAsSSE(sse *SSEWriter, chunk []byte) (bool, error) {
	lines := strings.Split(string(chunk), "\n")
	doneWritten := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if payload == "[DONE]" {
				if err := sse.WriteDone(); err != nil {
					return doneWritten, err
				}
				doneWritten = true
				continue
			}
			if err := sse.WriteEvent(payload); err != nil {
				return doneWritten, err
			}
			continue
		}

		if err := sse.WriteEvent(trimmed); err != nil {
			return doneWritten, err
		}
	}

	return doneWritten, nil
}

func isBodyTooLarge(err error) bool {
	if err == nil {
		return false
	}

	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return true
	}
	return errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(strings.ToLower(err.Error()), "request body too large")
}

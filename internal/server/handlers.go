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
	"github.com/rs/zerolog/log"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Warn().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("health endpoint rejected invalid method")
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		log.Warn().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("models endpoint rejected invalid method")
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}

	acct, ok := accountFromContext(r.Context())
	if !ok {
		log.Error().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("models endpoint missing account context")
		writeAPIError(w, http.StatusUnauthorized, "missing account context", "invalid_request_error", "invalid_api_key")
		return
	}

	log.Debug().
		Str("account_uuid", acct.UUID).
		Msg("serving models list")

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
		log.Warn().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("chat completions endpoint rejected invalid method")
		writeAPIError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}

	acct, ok := accountFromContext(r.Context())
	if !ok {
		log.Error().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("chat completions endpoint missing account context")
		writeAPIError(w, http.StatusUnauthorized, "missing account context", "invalid_request_error", "invalid_api_key")
		return
	}

	var reqBody types.ChatCompletionRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reqBody); err != nil {
		if isBodyTooLarge(err) {
			log.Warn().
				Err(err).
				Str("account_uuid", acct.UUID).
				Msg("chat completions request body too large")
			writeAPIError(w, http.StatusRequestEntityTooLarge, "request body too large", "invalid_request_error", "request_too_large")
			return
		}
		log.Warn().
			Err(err).
			Str("account_uuid", acct.UUID).
			Msg("chat completions invalid request body")
		writeAPIError(w, http.StatusBadRequest, "invalid request body", "invalid_request_error", "bad_request")
		return
	}
	if reqBody.Model == "" || len(reqBody.Messages) == 0 {
		log.Warn().
			Str("account_uuid", acct.UUID).
			Msg("chat completions missing required fields")
		writeAPIError(w, http.StatusBadRequest, "model and messages are required", "invalid_request_error", "bad_request")
		return
	}

	log.Debug().
		Str("account_uuid", acct.UUID).
		Str("model", reqBody.Model).
		Bool("stream", reqBody.Stream).
		Int("messages", len(reqBody.Messages)).
		Msg("chat completions request accepted")

	client := s.newProxy(acct)
	if reqBody.Stream {
		s.handleStreamChatCompletions(r.Context(), w, client, &reqBody, acct.UUID)
		return
	}

	resp, err := client.ChatCompletions(r.Context(), &reqBody)
	if err != nil {
		log.Warn().
			Err(err).
			Str("account_uuid", acct.UUID).
			Str("model", reqBody.Model).
			Msg("chat completions upstream request failed")
		writeAPIError(w, http.StatusBadGateway, fmt.Sprintf("upstream request failed: %v", err), "api_error", "upstream_error")
		return
	}

	if err := s.accountMgr.UpdateUsage(acct.UUID); err != nil {
		log.Warn().
			Err(err).
			Str("account_uuid", acct.UUID).
			Msg("failed to update account usage")
	}
	log.Debug().
		Str("account_uuid", acct.UUID).
		Str("model", reqBody.Model).
		Msg("chat completions response returned")
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStreamChatCompletions(ctx context.Context, w http.ResponseWriter, client proxyClient, reqBody *types.ChatCompletionRequest, uuid string) {
	stream, err := client.ChatCompletionsStream(ctx, reqBody)
	if err != nil {
		log.Warn().
			Err(err).
			Str("account_uuid", uuid).
			Str("model", reqBody.Model).
			Msg("chat completions stream request failed")
		writeAPIError(w, http.StatusBadGateway, fmt.Sprintf("upstream stream failed: %v", err), "api_error", "upstream_error")
		return
	}

	sse, err := NewSSEWriter(w)
	if err != nil {
		log.Error().
			Err(err).
			Str("account_uuid", uuid).
			Msg("failed to initialize sse writer")
		writeAPIError(w, http.StatusInternalServerError, err.Error(), "internal_error", "internal_error")
		return
	}

	doneWritten := false
	for {
		select {
		case <-ctx.Done():
			log.Debug().
				Str("account_uuid", uuid).
				Str("model", reqBody.Model).
				Msg("chat completions stream cancelled by context")
			return
		case chunk, ok := <-stream:
			if !ok {
				if !doneWritten {
					_ = sse.WriteDone()
				}
				if err := s.accountMgr.UpdateUsage(uuid); err != nil {
					log.Warn().
						Err(err).
						Str("account_uuid", uuid).
						Msg("failed to update account usage")
				}
				log.Debug().
					Str("account_uuid", uuid).
					Str("model", reqBody.Model).
					Msg("chat completions stream finished")
				return
			}

			wroteDone, writeErr := writeProxyChunkAsSSE(sse, chunk)
			if writeErr != nil {
				log.Warn().
					Err(writeErr).
					Str("account_uuid", uuid).
					Str("model", reqBody.Model).
					Msg("chat completions stream write failed")
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

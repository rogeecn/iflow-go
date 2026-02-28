package server

import (
	"fmt"
	"net/http"
)

type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("sse: response writer does not support flush")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	return &SSEWriter{
		w:       w,
		flusher: flusher,
	}, nil
}

func (s *SSEWriter) WriteEvent(data string) error {
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("sse write event: %w", err)
	}
	s.flusher.Flush()
	return nil
}

func (s *SSEWriter) WriteDone() error {
	if _, err := s.w.Write([]byte("data: [DONE]\n\n")); err != nil {
		return fmt.Errorf("sse write done: %w", err)
	}
	s.flusher.Flush()
	return nil
}

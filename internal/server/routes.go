package server

import "net/http"

func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("/health", chain(
		http.HandlerFunc(s.handleHealth),
		LoggingMiddleware,
	))

	mux.Handle("/v1/models", chain(
		http.HandlerFunc(s.handleModels),
		LoggingMiddleware,
		AuthMiddleware(s.accountMgr),
	))

	mux.Handle("/v1/chat/completions", chain(
		http.HandlerFunc(s.handleChatCompletions),
		LoggingMiddleware,
		AuthMiddleware(s.accountMgr),
		RequestSizeLimitMiddleware(defaultMaxBodySize),
	))

	return mux
}

func chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

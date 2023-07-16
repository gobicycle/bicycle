package api

import (
	"errors"
	"github.com/gobicycle/bicycle/internal/oas"
	"go.uber.org/zap"
	"net/http"
)

type Server struct {
	logger     *zap.Logger
	httpServer *http.Server
	mux        *http.ServeMux
}

type ServerOptions struct {
	ogenMiddlewares []oas.Middleware
}

type ServerOption func(options *ServerOptions)

func NewServer(log *zap.Logger, handler *Handler, token string, address string, opts ...ServerOption) (*Server, error) {

	options := &ServerOptions{}
	for _, o := range opts {
		o(options)
	}

	ogenMiddlewares := []oas.Middleware{ogenLoggingMiddleware(log)}
	ogenMiddlewares = append(ogenMiddlewares, options.ogenMiddlewares...)

	secHandler := NewSecurityHandler(token)

	ogenServer, err := oas.NewServer(handler, secHandler,
		oas.WithMiddleware(ogenMiddlewares...),
		oas.WithErrorHandler(ogenErrorsHandler))
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	mux.Handle("/", ogenServer)

	serv := Server{
		logger: log,
		mux:    mux,
		httpServer: &http.Server{
			Addr:    address,
			Handler: mux,
		},
	}

	return &serv, nil
}

func (s *Server) Run() {
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		s.logger.Info("server quit")
		return
	}
	s.logger.Fatal("ListedAndServe() failed", zap.Error(err))
}

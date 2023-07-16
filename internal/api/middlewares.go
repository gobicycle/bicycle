package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ogen-go/ogen/middleware"
	"go.uber.org/zap"
	"net/http"
)

var ErrRateLimit = errors.New("rate limit")

type errorJSON struct {
	Error string
}

func ogenLoggingMiddleware(logger *zap.Logger) middleware.Middleware {
	return func(req middleware.Request, next middleware.Next) (middleware.Response, error) {
		logger := logger.With(
			zap.String("operation", req.OperationName),
			zap.String("path", req.Raw.URL.Path),
		)
		logger.Info("Handling request")
		resp, err := next(req)
		if err != nil {
			logger.Error("Fail", zap.Error(err))
		} else {
			var fields []zap.Field
			if tresp, ok := resp.Type.(interface{ GetStatusCode() int }); ok {
				fields = []zap.Field{
					zap.Int("status_code", tresp.GetStatusCode()),
				}
			}
			logger.Info("Success", fields...)
		}
		return resp, err
	}
}

func ogenErrorsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("content-type", "application/json")
	if errors.Is(err, ErrInvalidToken) {
		w.WriteHeader(http.StatusUnauthorized)
	} else if errors.Is(err, ErrRateLimit) {
		w.WriteHeader(http.StatusTooManyRequests)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(&errorJSON{Error: err.Error()})
}

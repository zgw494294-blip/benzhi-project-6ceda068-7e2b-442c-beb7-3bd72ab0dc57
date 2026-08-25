package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"benzhi-project-6ceda068-7e2b-442c-beb7-3bd72ab0dc57/internal/archive"
)

type contextKey string

const correlationKey contextKey = "correlation-id"

var requestCounter atomic.Uint64

func (a *API) requestMetadata(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		correlation := strings.TrimSpace(request.Header.Get("X-Correlation-ID"))
		if correlation == "" || len(correlation) > 160 {
			correlation = fmt.Sprintf("http_%d_%d", time.Now().UnixNano(), requestCounter.Add(1))
		}
		w.Header().Set("X-Correlation-ID", correlation)
		w.Header().Set("Cache-Control", "no-store")
		ctx := context.WithValue(request.Context(), correlationKey, correlation)
		next.ServeHTTP(w, request.WithContext(ctx))
	})
}

func (a *API) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				_ = debug.Stack()
				writeError(w, request, &archive.Error{Code: "INTERNAL_ERROR", Message: "服务处理请求时发生内部错误"})
			}
		}()
		next.ServeHTTP(w, request)
	})
}

func correlationID(request *http.Request) string {
	value, _ := request.Context().Value(correlationKey).(string)
	return value
}

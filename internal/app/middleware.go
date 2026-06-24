// This file assembles shared HTTP middleware for Saturn routes.
package app

import (
	"net/http"
	"time"

	"github.com/stark-lin/saturn/internal/platform/httpx"
)

func (a *App) withMiddleware(next http.Handler) http.Handler {
	return httpx.Recover(a.Logger)(
		httpx.RequestID(
			httpx.CaptureRequestSource(a.Config.HTTP.TrustedProxyCIDRs, a.requestLog(next)),
		),
	)
}

func (a *App) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		a.Logger.Info("request completed", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
	})
}

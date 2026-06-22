// This file manages Saturn server startup and graceful shutdown.
package app

import (
	"context"
	"errors"
	"net/http"
)

func (a *App) Run(ctx context.Context) error {
	if a.LLMWorker != nil {
		go func() {
			if err := a.LLMWorker.Run(ctx); err != nil {
				a.Logger.Error("llm worker stopped", "error", err)
			}
		}()
	}
	a.Logger.Info("starting server", "addr", a.Config.HTTP.Addr)
	err := a.Server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (a *App) Shutdown(ctx context.Context) error {
	a.Logger.Info("shutting down server")
	a.Events.Close()
	return errors.Join(
		a.Server.Shutdown(ctx),
		a.Database.Close(),
		a.Redis.Close(),
	)
}

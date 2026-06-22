// This file runs fixed-concurrency PostgreSQL-backed LLM request workers.
package llm

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	defaultWorkerCount  = 1
	defaultPollInterval = time.Second
)

type RequestProcessor interface {
	ProcessNextQueuedRequest(ctx context.Context, requestTimeout time.Duration) (bool, error)
}

type WorkerLogger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type WorkerConfig struct {
	WorkerCount    int
	PollInterval   time.Duration
	RequestTimeout time.Duration
}

type Worker struct {
	processor      RequestProcessor
	logger         WorkerLogger
	workerCount    int
	pollInterval   time.Duration
	requestTimeout time.Duration
}

func NewWorker(processor RequestProcessor, config WorkerConfig, logger WorkerLogger) *Worker {
	workerCount := config.WorkerCount
	if workerCount < 1 {
		workerCount = defaultWorkerCount
	}
	pollInterval := config.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	return &Worker{
		processor:      processor,
		logger:         logger,
		workerCount:    workerCount,
		pollInterval:   pollInterval,
		requestTimeout: config.RequestTimeout,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	if w == nil || w.processor == nil {
		return ErrDependencyUnavailable
	}
	w.logInfo("starting llm workers", "worker_count", w.workerCount)
	var waitGroup sync.WaitGroup
	waitGroup.Add(w.workerCount)
	for workerID := 1; workerID <= w.workerCount; workerID++ {
		go func(workerID int) {
			defer waitGroup.Done()
			w.runLoop(ctx, workerID)
		}(workerID)
	}
	waitGroup.Wait()
	return nil
}

func (w *Worker) runLoop(ctx context.Context, workerID int) {
	backoff := w.pollInterval
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		processed, err := w.processor.ProcessNextQueuedRequest(ctx, w.requestTimeout)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if err != nil {
			w.logError("llm worker request failed", "worker_id", workerID, "error", err)
			if !sleepWithContext(ctx, backoff) {
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		backoff = w.pollInterval

		if processed {
			continue
		}
		if !sleepWithContext(ctx, w.pollInterval) {
			return
		}
	}
}

func sleepWithContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (w *Worker) logInfo(msg string, args ...any) {
	if w.logger != nil {
		w.logger.Info(msg, args...)
	}
}

func (w *Worker) logError(msg string, args ...any) {
	if w.logger != nil {
		w.logger.Error(msg, args...)
	}
}

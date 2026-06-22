// This file tests fixed-concurrency LLM worker behavior.
package llm

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestWorkerLimitsConcurrentRequests(t *testing.T) {
	processor := &concurrencyProcessor{
		total: 4,
		done:  make(chan struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker := NewWorker(processor, WorkerConfig{
		WorkerCount:  2,
		PollInterval: time.Millisecond,
	}, nil)
	runErr := make(chan error, 1)
	go func() {
		runErr <- worker.Run(ctx)
	}()

	select {
	case <-processor.done:
	case <-time.After(time.Second):
		t.Fatal("worker did not process expected requests")
	}
	cancel()
	if err := <-runErr; err != nil {
		t.Fatalf("worker run error = %v", err)
	}
	if processor.maxConcurrent > 2 {
		t.Fatalf("max concurrent requests = %d, want <= 2", processor.maxConcurrent)
	}
	if processor.maxConcurrent < 2 {
		t.Fatalf("max concurrent requests = %d, want worker concurrency to be used", processor.maxConcurrent)
	}
}

type concurrencyProcessor struct {
	mu            sync.Mutex
	total         int
	started       int
	finished      int
	current       int
	maxConcurrent int
	done          chan struct{}
	closed        bool
}

func (p *concurrencyProcessor) ProcessNextQueuedRequest(ctx context.Context, _ time.Duration) (bool, error) {
	p.mu.Lock()
	if p.started >= p.total {
		p.mu.Unlock()
		return false, nil
	}
	p.started++
	p.current++
	if p.current > p.maxConcurrent {
		p.maxConcurrent = p.current
	}
	p.mu.Unlock()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(20 * time.Millisecond):
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.current--
	p.finished++
	if p.finished == p.total && !p.closed {
		close(p.done)
		p.closed = true
	}
	return true, nil
}

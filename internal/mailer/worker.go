package mailer

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Worker is an in-process async email dispatcher (parity with Laravel's queued
// Mail::queue). It implements Mailer: Send puts the message on a bounded
// channel and returns immediately; a single goroutine drains the channel and
// sends with retries. No Redis/broker is added (FASE 3 decision).
//
// Shutdown is graceful: close the channel, let the worker finish the backlog,
// then return. Emails that exhaust retries are logged by subject/recipient —
// never the body (which may contain the OTP).
type Worker struct {
	queue     chan Message
	mailer    Mailer
	logger    *slog.Logger
	closeOnce sync.Once
	wg        sync.WaitGroup
}

const (
	defaultQueueSize = 64
	maxAttempts      = 3
)

func NewWorker(m Mailer, logger *slog.Logger) *Worker {
	return &Worker{queue: make(chan Message, defaultQueueSize), mailer: m, logger: logger}
}

// Start launches the consume loop. Call once at startup.
func (w *Worker) Start() {
	w.wg.Add(1)
	go w.loop()
}

// Send enqueues a message asynchronously. It blocks only while the bounded
// queue is full (backpressure), or returns ctx.Err on cancellation.
func (w *Worker) Send(ctx context.Context, m Message) error {
	select {
	case w.queue <- m:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown stops accepting new messages and waits for the backlog to drain.
func (w *Worker) Shutdown(ctx context.Context) error {
	w.closeOnce.Do(func() { close(w.queue) })
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Worker) loop() {
	defer w.wg.Done()
	for m := range w.queue {
		w.sendWithRetry(m)
	}
}

func (w *Worker) sendWithRetry(m Message) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := w.mailer.Send(context.Background(), m)
		if err == nil {
			return
		}
		w.logger.Error("mail_send_failed",
			"attempt", attempt,
			"to", m.ToEmail,
			"subject", m.Subject,
			"error", err.Error(),
		)
		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	w.logger.Error("mail_dropped", "to", m.ToEmail, "subject", m.Subject)
}

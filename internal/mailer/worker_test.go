package mailer

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

type recordingMailer struct {
	mu         sync.Mutex
	msgs       []Message
	alwaysFail bool
	calls      int
}

func (m *recordingMailer) Send(ctx context.Context, msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.alwaysFail {
		return &diagErr{msg: "smtp down"}
	}
	m.msgs = append(m.msgs, msg)
	return nil
}

type diagErr struct{ msg string }

func (e *diagErr) Error() string { return e.msg }

func TestWorkerSendsEnqueuedEmails(t *testing.T) {
	inner := &recordingMailer{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWorker(inner, logger)
	w.Start()

	n := 10
	for i := 0; i < n; i++ {
		if err := w.Send(context.Background(), Message{ToEmail: "a@example.test", Subject: "x"}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if len(inner.msgs) != n {
		t.Fatalf("expected %d delivered, got %d", n, len(inner.msgs))
	}
}

func TestWorkerDropsMailAfterRetries(t *testing.T) {
	inner := &recordingMailer{alwaysFail: true}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	w := NewWorker(inner, logger)
	w.Start()

	if err := w.Send(context.Background(), Message{ToEmail: "a@example.test", Subject: "x"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	inner.mu.Lock()
	defer inner.mu.Unlock()
	if inner.calls != maxAttempts {
		t.Fatalf("expected %d send attempts before drop, got %d", maxAttempts, inner.calls)
	}
	if len(inner.msgs) != 0 {
		t.Fatal("no message should have been delivered")
	}
}

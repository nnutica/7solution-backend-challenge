package app_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nnutica/7solution-backend-challenge/internal/app"
)

type stubCounter struct {
	calls chan struct{}
	err   error
}

func (s *stubCounter) Count(context.Context) (int64, error) {
	// Non-blocking so extra ticks never stall the reporter.
	select {
	case s.calls <- struct{}{}:
	default:
	}
	return 42, s.err
}

// syncBuffer guards a bytes.Buffer because the reporter writes from another goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestReportUserCount(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantLog string
	}{
		{"logs count", nil, `"total":42`},
		{"keeps running on error", errors.New("db down"), `"error":"db down"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := &stubCounter{calls: make(chan struct{}, 10), err: tt.err}
			var out syncBuffer
			log := slog.New(slog.NewJSONHandler(&out, nil))
			ctx, cancel := context.WithCancel(context.Background())

			done := make(chan struct{})
			go func() {
				app.ReportUserCount(ctx, counter, 5*time.Millisecond, log)
				close(done)
			}()

			// Two ticks prove it repeats, including after an error.
			for range 2 {
				select {
				case <-counter.calls:
				case <-time.After(time.Second):
					t.Fatal("reporter did not tick")
				}
			}

			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("reporter did not stop after cancel")
			}

			if !strings.Contains(out.String(), tt.wantLog) {
				t.Errorf("log %q does not contain %q", out.String(), tt.wantLog)
			}
		})
	}
}

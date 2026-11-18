package app

import (
	"context"
	"log/slog"
	"time"
)

type userCounter interface {
	Count(ctx context.Context) (int64, error)
}

// ReportUserCount logs the total number of users every interval until ctx is cancelled.
func ReportUserCount(ctx context.Context, counter userCounter, interval time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("user count reporter stopped")
			return
		case <-ticker.C:
			// Bound each query so a slow database cannot pile up ticks.
			countCtx, cancel := context.WithTimeout(ctx, interval/2)
			n, err := counter.Count(countCtx)
			cancel()
			if err != nil {
				log.Error("count users", "error", err)
				continue
			}
			log.Info("user count", "total", n)
		}
	}
}

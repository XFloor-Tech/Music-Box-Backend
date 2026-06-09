package auth

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	expiredSessionCleanupTimeout = 10 * time.Second
)

func (m *Module) StartExpiredSessionCleanup(ctx context.Context, logger *zap.Logger) func() {
	if m == nil || m.storer == nil {
		return func() {}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)

		m.deleteExpiredSessions(ctx, logger)

		ticker := time.NewTicker(m.sessionCleanupInterval())
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.deleteExpiredSessions(ctx, logger)
			}
		}
	}()

	var stopOnce sync.Once
	return func() {
		stopOnce.Do(func() {
			cancel()
			<-done
		})
	}
}

func (m *Module) sessionCleanupInterval() time.Duration {
	if m == nil || m.config.SessionCleanupInterval <= 0 {
		return defaultSessionCleanupInterval
	}

	return m.config.SessionCleanupInterval
}

func (m *Module) deleteExpiredSessions(ctx context.Context, logger *zap.Logger) {
	cleanupCtx, cancel := context.WithTimeout(ctx, expiredSessionCleanupTimeout)
	defer cancel()

	deleted, err := m.storer.DeleteExpiredSessions(cleanupCtx)
	if err != nil {
		logger.Warn("failed to delete expired auth sessions", zap.Error(err))
		return
	}

	logger.Info("deleted expired auth sessions", zap.Int64("count", deleted))
}

func (s *PostgresStorer) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.repo.Exec(ctx, `
DELETE FROM "session"
WHERE "expiresAt" <= NOW()
`)
	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil
}

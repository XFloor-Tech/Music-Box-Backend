package auth

import (
	"context"
	"strings"
	"testing"
)

func TestDeleteExpiredSessionsDeletesOnlyExpiredRows(t *testing.T) {
	repo := &recordingRepo{}
	storer := NewPostgresStorer(repo)

	deleted, err := storer.DeleteExpiredSessions(context.Background())
	if err != nil {
		t.Fatalf("DeleteExpiredSessions() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if len(repo.execQueries) != 1 {
		t.Fatalf("Exec calls = %d, want 1", len(repo.execQueries))
	}

	query := strings.Join(strings.Fields(repo.execQueries[0]), " ")
	if query != `DELETE FROM "session" WHERE "expiresAt" <= NOW()` {
		t.Fatalf("delete query = %q, want expired-session cleanup", query)
	}
	if len(repo.execArgs[0]) != 0 {
		t.Fatalf("delete args = %#v, want none", repo.execArgs[0])
	}
}

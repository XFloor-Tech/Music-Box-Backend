package track

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestEnsureSchemaRepairsLegacyTrackColumnsBeforeIndexes(t *testing.T) {
	repo := &recordingRepository{}
	tracks := NewPostgresRepository(repo)

	if err := tracks.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	trackTableIndex := indexOfStatement(repo.statements, `CREATE TABLE IF NOT EXISTS "track"`)
	trackRenameIndex := indexOfStatement(repo.statements, `RENAME COLUMN "release_year" TO "releaseYear"`)
	trackAddIndex := indexOfStatement(repo.statements, `ADD COLUMN IF NOT EXISTS "releaseYear" INTEGER`)
	trackIndexIndex := indexOfStatement(repo.statements, `track_user_created_at_id_idx`)
	if trackTableIndex == -1 || trackRenameIndex == -1 || trackAddIndex == -1 || trackIndexIndex == -1 {
		t.Fatalf("track schema statements missing: table=%d rename=%d add=%d index=%d", trackTableIndex, trackRenameIndex, trackAddIndex, trackIndexIndex)
	}
	if !(trackTableIndex < trackRenameIndex && trackRenameIndex < trackAddIndex && trackAddIndex < trackIndexIndex) {
		t.Fatalf("track schema order = table:%d rename:%d add:%d index:%d, want table < rename < add < index", trackTableIndex, trackRenameIndex, trackAddIndex, trackIndexIndex)
	}

	mediaTableIndex := indexOfStatement(repo.statements, `CREATE TABLE IF NOT EXISTS track_media`)
	mediaRenameIndex := indexOfStatement(repo.statements, `RENAME COLUMN "track_id" TO "trackId"`)
	mediaAddIndex := indexOfStatement(repo.statements, `ADD COLUMN IF NOT EXISTS "storageProvider" track_storage_provider`)
	mediaIndexIndex := indexOfStatement(repo.statements, `track_media_track_id_idx`)
	if mediaTableIndex == -1 || mediaRenameIndex == -1 || mediaAddIndex == -1 || mediaIndexIndex == -1 {
		t.Fatalf("track media schema statements missing: table=%d rename=%d add=%d index=%d", mediaTableIndex, mediaRenameIndex, mediaAddIndex, mediaIndexIndex)
	}
	if !(mediaTableIndex < mediaRenameIndex && mediaRenameIndex < mediaAddIndex && mediaAddIndex < mediaIndexIndex) {
		t.Fatalf("track media schema order = table:%d rename:%d add:%d index:%d, want table < rename < add < index", mediaTableIndex, mediaRenameIndex, mediaAddIndex, mediaIndexIndex)
	}
}

func TestTrackQueriesUseCamelCaseDatabaseColumns(t *testing.T) {
	for _, fragment := range []string{
		`"userId"`,
		`"releaseYear"`,
		`"trackNumber"`,
		`"discNumber"`,
		`"durationMs"`,
		`"createdAt"`,
		`"updatedAt"`,
	} {
		if !strings.Contains(trackSelectColumns, fragment) {
			t.Fatalf("trackSelectColumns does not contain %s", fragment)
		}
	}

	repo := &recordingRepository{}
	tracks := NewPostgresRepository(repo)
	_, _ = tracks.ListByUserID(context.Background(), "usr_123", ListTracksOptions{
		Limit: 20,
		Cursor: &TrackListCursor{
			CreatedAt: time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
			ID:        "trk_123",
		},
	})

	query := repo.lastQuery
	for _, fragment := range []string{
		`"userId" = $1`,
		`("createdAt", id)`,
		`ORDER BY "createdAt" DESC`,
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("list query does not contain %s:\n%s", fragment, query)
		}
	}
}

func indexOfStatement(statements []string, needle string) int {
	for i, statement := range statements {
		if strings.Contains(statement, needle) {
			return i
		}
	}

	return -1
}

type recordingRepository struct {
	statements []string
	lastQuery  string
}

func (r *recordingRepository) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	r.statements = append(r.statements, query)
	return pgconn.CommandTag{}, nil
}

func (r *recordingRepository) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	r.lastQuery = query
	return emptyRows{}, nil
}

func (r *recordingRepository) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return nil
}

type emptyRows struct{}

func (emptyRows) Close()                                       {}
func (emptyRows) Err() error                                   { return nil }
func (emptyRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (emptyRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (emptyRows) Next() bool                                   { return false }
func (emptyRows) Scan(dest ...any) error                       { return nil }
func (emptyRows) Values() ([]any, error)                       { return nil, nil }
func (emptyRows) RawValues() [][]byte                          { return nil }
func (emptyRows) Conn() *pgx.Conn                              { return nil }

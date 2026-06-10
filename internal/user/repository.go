package user

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"xfloor/music-box-backend/internal/database"
)

var ErrUserNotFound = errors.New("user not found")

type Profile struct {
	ID            string
	Email         string
	Name          string
	EmailVerified bool
	Image         string
}

type Repository interface {
	LoadProfileByID(ctx context.Context, id string) (Profile, error)
}

type PostgresRepository struct {
	repo database.Repository
}

func NewPostgresRepository(repo database.Repository) *PostgresRepository {
	return &PostgresRepository{repo: repo}
}

func (r *PostgresRepository) LoadProfileByID(ctx context.Context, id string) (Profile, error) {
	id = strings.TrimSpace(id)

	if id == "" {
		return Profile{}, ErrUserNotFound
	}

	var profile Profile
	err := r.repo.QueryRow(ctx, `
SELECT id, email, name, "emailVerified", COALESCE(image, '')
FROM "user"
WHERE id = $1
`, id).Scan(
		&profile.ID,
		&profile.Email,
		&profile.Name,
		&profile.EmailVerified,
		&profile.Image,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrUserNotFound
	}

	if err != nil {
		return Profile{}, err
	}

	return profile, nil
}

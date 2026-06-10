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

type UpdateProfileInput struct {
	Name  *string
	Image *string
}

func (input UpdateProfileInput) Empty() bool {
	return input.Name == nil && input.Image == nil
}

type Repository interface {
	LoadProfileByID(ctx context.Context, id string) (Profile, error)
	UpdateProfileByID(ctx context.Context, id string, input UpdateProfileInput) (Profile, error)
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

func (r *PostgresRepository) UpdateProfileByID(ctx context.Context, id string, input UpdateProfileInput) (Profile, error) {
	id = strings.TrimSpace(id)

	if id == "" || input.Empty() {
		return Profile{}, ErrUserNotFound
	}

	var profile Profile
	err := r.repo.QueryRow(ctx, `
UPDATE "user"
SET name = COALESCE($2, name),
	image = COALESCE($3, image),
	"updatedAt" = NOW()
WHERE id = $1
RETURNING id, email, name, "emailVerified", COALESCE(image, '')
`, id, optionalStringArg(input.Name), optionalStringArg(input.Image)).Scan(
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

func optionalStringArg(value *string) any {
	if value == nil {
		return nil
	}

	return *value
}

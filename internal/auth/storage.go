package auth

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aarondl/authboss/v3"
	"github.com/jackc/pgx/v5"

	"xfloor/music-box-backend/internal/database"
)

const credentialProviderID = "credential"

const (
	createUserTableSQL = `
CREATE TABLE IF NOT EXISTS "user" (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL UNIQUE,
	"emailVerified" BOOLEAN NOT NULL DEFAULT FALSE,
	image TEXT,
	"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

	createSessionTableSQL = `
CREATE TABLE IF NOT EXISTS "session" (
	id TEXT PRIMARY KEY,
	"userId" TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
	token TEXT NOT NULL UNIQUE,
	"expiresAt" TIMESTAMPTZ NOT NULL,
	"ipAddress" TEXT,
	"userAgent" TEXT,
	"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

	createAccountTableSQL = `
CREATE TABLE IF NOT EXISTS "account" (
	id TEXT PRIMARY KEY,
	"userId" TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
	"accountId" TEXT NOT NULL,
	"providerId" TEXT NOT NULL,
	"accessToken" TEXT,
	"refreshToken" TEXT,
	"idToken" TEXT,
	"accessTokenExpiresAt" TIMESTAMPTZ,
	"refreshTokenExpiresAt" TIMESTAMPTZ,
	scope TEXT,
	password TEXT,
	"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE ("providerId", "accountId")
)`

	createVerificationTableSQL = `
CREATE TABLE IF NOT EXISTS "verification" (
	id TEXT PRIMARY KEY,
	identifier TEXT NOT NULL,
	value TEXT NOT NULL,
	"expiresAt" TIMESTAMPTZ NOT NULL,
	"createdAt" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"updatedAt" TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

	createSessionUserIndexSQL       = `CREATE INDEX IF NOT EXISTS session_user_id_idx ON "session" ("userId")`
	createSessionExpiresIndexSQL    = `CREATE INDEX IF NOT EXISTS session_expires_at_idx ON "session" ("expiresAt")`
	createAccountUserIndexSQL       = `CREATE INDEX IF NOT EXISTS account_user_id_idx ON "account" ("userId")`
	createVerificationIdentifierSQL = `CREATE INDEX IF NOT EXISTS verification_identifier_idx ON "verification" (identifier)`
)

type User struct {
	ID            string
	PID           string
	Name          string
	EmailVerified bool
	Image         string
	Password      string
	Arbitrary     map[string]string
}

var _ authboss.AuthableUser = (*User)(nil)
var _ authboss.ArbitraryUser = (*User)(nil)

func (u *User) GetPID() string {
	return u.PID
}

func (u *User) PutPID(pid string) {
	u.PID = normalizeEmail(pid)
}

func (u *User) GetPassword() string {
	return u.Password
}

func (u *User) PutPassword(password string) {
	u.Password = password
}

func (u *User) GetArbitrary() map[string]string {
	return u.Arbitrary
}

func (u *User) PutArbitrary(arbitrary map[string]string) {
	u.Arbitrary = arbitrary
}

type Session struct {
	ID        string
	UserID    string
	Token     string
	ExpiresAt time.Time
	IPAddress string
	UserAgent string
}

type PostgresStorer struct {
	repo database.Repository
}

var _ authboss.CreatingServerStorer = (*PostgresStorer)(nil)
var _ authboss.RememberingServerStorer = (*PostgresStorer)(nil)

func NewPostgresStorer(repo database.Repository) *PostgresStorer {
	return &PostgresStorer{repo: repo}
}

func (s *PostgresStorer) EnsureSchema(ctx context.Context) error {
	statements := []string{
		createUserTableSQL,
		createSessionTableSQL,
		createAccountTableSQL,
		createVerificationTableSQL,
		createSessionUserIndexSQL,
		createSessionExpiresIndexSQL,
		createAccountUserIndexSQL,
		createVerificationIdentifierSQL,
	}

	for _, statement := range statements {
		if _, err := s.repo.Exec(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}

func (s *PostgresStorer) New(ctx context.Context) authboss.User {
	return &User{}
}

func (s *PostgresStorer) Load(ctx context.Context, key string) (authboss.User, error) {
	email := normalizeEmail(key)
	if email == "" {
		return nil, authboss.ErrUserNotFound
	}

	user := &User{}
	err := s.repo.QueryRow(ctx, `
SELECT u.id, u.email, u.name, u."emailVerified", COALESCE(u.image, ''), a.password
FROM "user" u
JOIN "account" a ON a."userId" = u.id AND a."providerId" = $2
WHERE u.email = $1
`, email, credentialProviderID).Scan(
		&user.ID,
		&user.PID,
		&user.Name,
		&user.EmailVerified,
		&user.Image,
		&user.Password,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authboss.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *PostgresStorer) LoadByID(ctx context.Context, id string) (*User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, authboss.ErrUserNotFound
	}

	user := &User{}
	err := s.repo.QueryRow(ctx, `
SELECT u.id, u.email, u.name, u."emailVerified", COALESCE(u.image, ''), a.password
FROM "user" u
JOIN "account" a ON a."userId" = u.id AND a."providerId" = $2
WHERE u.id = $1
`, id, credentialProviderID).Scan(
		&user.ID,
		&user.PID,
		&user.Name,
		&user.EmailVerified,
		&user.Image,
		&user.Password,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authboss.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *PostgresStorer) Save(ctx context.Context, user authboss.User) error {
	authUser := authboss.MustBeAuthable(user)
	email := normalizeEmail(authUser.GetPID())
	if email == "" {
		return authboss.ErrUserNotFound
	}

	stored, err := s.userForSave(ctx, user, email)
	if err != nil {
		return err
	}

	tag, err := s.repo.Exec(ctx, `
UPDATE "account"
SET password = $2, "updatedAt" = NOW()
WHERE "userId" = $1 AND "providerId" = $3
`, stored.ID, authUser.GetPassword(), credentialProviderID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return authboss.ErrUserNotFound
	}

	_, err = s.repo.Exec(ctx, `
UPDATE "user"
SET email = $2, "updatedAt" = NOW()
WHERE id = $1
`, stored.ID, email)
	return err
}

func (s *PostgresStorer) Create(ctx context.Context, user authboss.User) error {
	authUser := authboss.MustBeAuthable(user)
	email := normalizeEmail(authUser.GetPID())
	if email == "" {
		return authboss.ErrUserNotFound
	}

	userID, err := randomID()
	if err != nil {
		return err
	}
	accountID, err := randomID()
	if err != nil {
		return err
	}

	name := displayNameFromEmail(email)
	if stored, ok := user.(*User); ok {
		stored.ID = userID
		stored.PID = email
		stored.Name = name
	}

	tag, err := s.repo.Exec(ctx, `
WITH inserted_user AS (
	INSERT INTO "user" (id, name, email, "emailVerified")
	VALUES ($1, $2, $3, FALSE)
	ON CONFLICT (email) DO NOTHING
	RETURNING id
)
INSERT INTO "account" (id, "userId", "accountId", "providerId", password)
SELECT $4, id, id, $5, $6
FROM inserted_user
`, userID, name, email, accountID, credentialProviderID, authUser.GetPassword())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return authboss.ErrUserFound
	}

	return nil
}

func (s *PostgresStorer) CreateSession(ctx context.Context, session Session) error {
	if session.ID == "" {
		id, err := randomID()
		if err != nil {
			return err
		}
		session.ID = id
	}

	_, err := s.repo.Exec(ctx, `
INSERT INTO "session" (id, "userId", token, "expiresAt", "ipAddress", "userAgent")
VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))
ON CONFLICT (token) DO UPDATE
SET "expiresAt" = EXCLUDED."expiresAt",
	"ipAddress" = EXCLUDED."ipAddress",
	"userAgent" = EXCLUDED."userAgent",
	"updatedAt" = NOW()
`, session.ID, session.UserID, session.Token, session.ExpiresAt, session.IPAddress, session.UserAgent)
	return err
}

func (s *PostgresStorer) ValidateSession(ctx context.Context, userID, token string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(token) == "" {
		return authboss.ErrTokenNotFound
	}

	var found int
	err := s.repo.QueryRow(ctx, `
SELECT 1
FROM "session"
WHERE "userId" = $1 AND token = $2 AND "expiresAt" > NOW()
`, userID, token).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return authboss.ErrTokenNotFound
	}

	return err
}

func (s *PostgresStorer) DeleteSession(ctx context.Context, userID, token string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(token) == "" {
		return nil
	}

	_, err := s.repo.Exec(ctx, `
DELETE FROM "session"
WHERE "userId" = $1 AND token = $2
`, userID, token)
	return err
}

func (s *PostgresStorer) AddRememberToken(ctx context.Context, pid, token string) error {
	return s.AddRememberTokenWithTTL(ctx, pid, token, defaultRefreshTokenTTL)
}

func (s *PostgresStorer) AddRememberTokenWithTTL(ctx context.Context, pid, token string, ttl time.Duration) error {
	user, err := s.loadAuthUser(ctx, pid)
	if err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = defaultRefreshTokenTTL
	}

	return s.CreateSession(ctx, Session{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().UTC().Add(ttl),
	})
}

func (s *PostgresStorer) DelRememberTokens(ctx context.Context, pid string) error {
	user, err := s.loadAuthUser(ctx, pid)
	if err != nil {
		if err == authboss.ErrUserNotFound {
			return nil
		}
		return err
	}

	_, err = s.repo.Exec(ctx, `
DELETE FROM "session"
WHERE "userId" = $1
`, user.ID)
	return err
}

func (s *PostgresStorer) UseRememberToken(ctx context.Context, pid, token string) error {
	user, err := s.loadAuthUser(ctx, pid)
	if err != nil {
		return err
	}

	tag, err := s.repo.Exec(ctx, `
DELETE FROM "session"
WHERE "userId" = $1 AND token = $2 AND "expiresAt" > NOW()
`, user.ID, token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return authboss.ErrTokenNotFound
	}

	return nil
}

func (s *PostgresStorer) UseRefreshToken(ctx context.Context, token string) (*User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, authboss.ErrTokenNotFound
	}

	var userID string
	err := s.repo.QueryRow(ctx, `
DELETE FROM "session"
WHERE token = $1 AND "expiresAt" > NOW()
RETURNING "userId"
`, token).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authboss.ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}

	return s.LoadByID(ctx, userID)
}

func (s *PostgresStorer) userForSave(ctx context.Context, user authboss.User, email string) (*User, error) {
	if stored, ok := user.(*User); ok && stored.ID != "" {
		return stored, nil
	}

	return s.loadAuthUser(ctx, email)
}

func (s *PostgresStorer) loadAuthUser(ctx context.Context, pid string) (*User, error) {
	user, err := s.Load(ctx, pid)
	if err != nil {
		return nil, err
	}

	stored, ok := user.(*User)
	if !ok {
		return nil, authboss.ErrUserNotFound
	}

	return stored, nil
}

func requestSessionMetadata(r *http.Request) (ipAddress, userAgent string) {
	if r == nil {
		return "", ""
	}

	ipAddress = strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(ipAddress); err == nil {
		ipAddress = host
	}

	return ipAddress, r.UserAgent()
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func displayNameFromEmail(email string) string {
	name, _, ok := strings.Cut(email, "@")
	if !ok || name == "" {
		return email
	}

	return name
}

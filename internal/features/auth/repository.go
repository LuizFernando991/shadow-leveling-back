package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*User, error)
	FindUserByEmail(ctx context.Context, email string) (*User, error)
	FindUserByID(ctx context.Context, id string) (*User, error)
	MarkUserVerified(ctx context.Context, email string) error

	CreateSession(ctx context.Context, userID, token, userAgent, ipAddress string, expiresAt *time.Time) (*Session, error)
	FindSessionByToken(ctx context.Context, token string) (*Session, error)
	FindSessionByID(ctx context.Context, id string) (*Session, error)
	ListSessionsByUserID(ctx context.Context, userID string) ([]*Session, error)
	RevokeSession(ctx context.Context, id string) error

	CreateEmailVerification(ctx context.Context, email, code string, vtype VerificationType, expiresAt time.Time) (*EmailVerification, error)
	FindEmailVerification(ctx context.Context, email, code string, vtype VerificationType) (*EmailVerification, error)
	DeleteEmailVerification(ctx context.Context, id string) error
	DeleteEmailVerificationsByEmailAndType(ctx context.Context, email string, vtype VerificationType) error
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

// ── Users ─────────────────────────────────────────────────────────────────────

func (r *postgresRepository) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, email, password_hash, verified_at, created_at, updated_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.VerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth: create user: %w", err)
	}
	return &u, nil
}

func (r *postgresRepository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, verified_at, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.VerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth: find user: %w", err)
	}
	return &u, nil
}

func (r *postgresRepository) FindUserByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, verified_at, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.VerifiedAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth: find user by id: %w", err)
	}
	return &u, nil
}

func (r *postgresRepository) MarkUserVerified(ctx context.Context, email string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET verified_at = NOW() WHERE email = $1 AND verified_at IS NULL`,
		email,
	)
	if err != nil {
		return fmt.Errorf("auth: mark user verified: %w", err)
	}
	return nil
}

// ── Sessions ──────────────────────────────────────────────────────────────────

func (r *postgresRepository) CreateSession(ctx context.Context, userID, token, userAgent, ipAddress string, expiresAt *time.Time) (*Session, error) {
	var s Session
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO sessions (user_id, token, user_agent, ip_address, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, token, user_agent, ip_address, expires_at, revoked_at, created_at`,
		userID, token, userAgent, ipAddress, expiresAt,
	).Scan(&s.ID, &s.UserID, &s.Token, &s.UserAgent, &s.IPAddress, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth: create session: %w", err)
	}
	return &s, nil
}

func (r *postgresRepository) FindSessionByToken(ctx context.Context, token string) (*Session, error) {
	var s Session
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, token, user_agent, ip_address, expires_at, revoked_at, created_at
		 FROM sessions WHERE token = $1`,
		token,
	).Scan(&s.ID, &s.UserID, &s.Token, &s.UserAgent, &s.IPAddress, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth: find session by token: %w", err)
	}
	return &s, nil
}

func (r *postgresRepository) FindSessionByID(ctx context.Context, id string) (*Session, error) {
	var s Session
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, token, user_agent, ip_address, expires_at, revoked_at, created_at
		 FROM sessions WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.UserID, &s.Token, &s.UserAgent, &s.IPAddress, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth: find session by id: %w", err)
	}
	return &s, nil
}

func (r *postgresRepository) ListSessionsByUserID(ctx context.Context, userID string) ([]*Session, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, token, user_agent, ip_address, expires_at, revoked_at, created_at
		 FROM sessions
		 WHERE user_id = $1 AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("auth: list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.Token, &s.UserAgent, &s.IPAddress, &s.ExpiresAt, &s.RevokedAt, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("auth: scan session: %w", err)
		}
		sessions = append(sessions, &s)
	}
	return sessions, rows.Err()
}

func (r *postgresRepository) RevokeSession(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("auth: revoke session: %w", err)
	}
	return nil
}

// ── Email verifications ───────────────────────────────────────────────────────

func (r *postgresRepository) CreateEmailVerification(ctx context.Context, email, code string, vtype VerificationType, expiresAt time.Time) (*EmailVerification, error) {
	var v EmailVerification
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO email_verifications (email, code, type, expires_at)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, code, type, expires_at, created_at`,
		email, code, string(vtype), expiresAt,
	).Scan(&v.ID, &v.Email, &v.Code, &v.Type, &v.ExpiresAt, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth: create email verification: %w", err)
	}
	return &v, nil
}

func (r *postgresRepository) FindEmailVerification(ctx context.Context, email, code string, vtype VerificationType) (*EmailVerification, error) {
	var v EmailVerification
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, code, type, expires_at, created_at
		 FROM email_verifications
		 WHERE email = $1 AND code = $2 AND type = $3 AND expires_at > NOW()
		 LIMIT 1`,
		email, code, string(vtype),
	).Scan(&v.ID, &v.Email, &v.Code, &v.Type, &v.ExpiresAt, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("auth: find email verification: %w", err)
	}
	return &v, nil
}

func (r *postgresRepository) DeleteEmailVerification(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM email_verifications WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("auth: delete email verification: %w", err)
	}
	return nil
}

func (r *postgresRepository) DeleteEmailVerificationsByEmailAndType(ctx context.Context, email string, vtype VerificationType) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM email_verifications WHERE email = $1 AND type = $2`,
		email, string(vtype),
	)
	if err != nil {
		return fmt.Errorf("auth: delete email verifications: %w", err)
	}
	return nil
}

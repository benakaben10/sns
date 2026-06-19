package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/benakaben10/sns/internal/model"
)

// ErrNotFound is returned when no SMTP config matches the query.
var ErrNotFound = errors.New("smtp config not found")

// SMTPConfigRepo is the interface used across the application.
type SMTPConfigRepo interface {
	GetByFromEmail(ctx context.Context, fromEmail string) (*model.SMTPConfig, error)
	GetDefault(ctx context.Context) (*model.SMTPConfig, error)
	List(ctx context.Context) ([]model.SMTPConfig, error)
	GetByID(ctx context.Context, id int64) (*model.SMTPConfig, error)
	Create(ctx context.Context, input model.SMTPConfigInput) (*model.SMTPConfig, error)
	Update(ctx context.Context, id int64, input model.SMTPConfigInput) (*model.SMTPConfig, error)
	Delete(ctx context.Context, id int64) error
	SetDefault(ctx context.Context, id int64) error
}

// SMTPConfigRepository is a PostgreSQL-backed SMTPConfigRepo.
type SMTPConfigRepository struct {
	db *pgxpool.Pool
}

// NewSMTPConfigRepository creates a repository backed by the given connection pool.
func NewSMTPConfigRepository(db *pgxpool.Pool) *SMTPConfigRepository {
	return &SMTPConfigRepository{db: db}
}

const smtpConfigColumns = `
	id, name, COALESCE(from_email, ''), host, port,
	username, password, use_tls, use_starttls, is_default,
	created_at, updated_at
`

// GetByFromEmail returns the SMTP config for the given sender address, or ErrNotFound.
func (r *SMTPConfigRepository) GetByFromEmail(ctx context.Context, fromEmail string) (*model.SMTPConfig, error) {
	row := r.db.QueryRow(ctx, `
		SELECT`+smtpConfigColumns+`
		FROM smtp_configs
		WHERE from_email = $1
		LIMIT 1
	`, fromEmail)
	cfg, err := scanSMTPConfig(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("lookup smtp config for %s: %w", fromEmail, ErrNotFound)
		}
		return nil, fmt.Errorf("query smtp config by from_email: %w", err)
	}
	return cfg, nil
}

// GetDefault returns the SMTP config marked as default, or ErrNotFound.
func (r *SMTPConfigRepository) GetDefault(ctx context.Context) (*model.SMTPConfig, error) {
	row := r.db.QueryRow(ctx, `
		SELECT`+smtpConfigColumns+`
		FROM smtp_configs
		WHERE is_default = true
		LIMIT 1
	`)
	cfg, err := scanSMTPConfig(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("query default smtp config: %w", err)
	}
	return cfg, nil
}

// List returns all SMTP configs ordered by id.
func (r *SMTPConfigRepository) List(ctx context.Context) ([]model.SMTPConfig, error) {
	rows, err := r.db.Query(ctx, `
		SELECT`+smtpConfigColumns+`
		FROM smtp_configs
		ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list smtp configs: %w", err)
	}
	defer rows.Close()

	var cfgs []model.SMTPConfig
	for rows.Next() {
		cfg, err := scanSMTPConfig(rows)
		if err != nil {
			return nil, fmt.Errorf("scan smtp config row: %w", err)
		}
		cfgs = append(cfgs, *cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate smtp config rows: %w", err)
	}
	return cfgs, nil
}

// GetByID returns the SMTP config with the given id, or ErrNotFound.
func (r *SMTPConfigRepository) GetByID(ctx context.Context, id int64) (*model.SMTPConfig, error) {
	row := r.db.QueryRow(ctx, `
		SELECT`+smtpConfigColumns+`
		FROM smtp_configs
		WHERE id = $1
	`, id)
	cfg, err := scanSMTPConfig(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get smtp config by id: %w", err)
	}
	return cfg, nil
}

// Create inserts a new SMTP config and returns the created record.
func (r *SMTPConfigRepository) Create(ctx context.Context, input model.SMTPConfigInput) (*model.SMTPConfig, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO smtp_configs
			(name, from_email, host, port, username, password, use_tls, use_starttls, is_default)
		VALUES ($1, NULLIF($2,''), $3, $4, $5, $6, $7, $8, $9)
		RETURNING`+smtpConfigColumns,
		input.Name, input.FromEmail, input.Host, input.Port,
		input.Username, input.Password,
		input.UseTLS, input.UseSTARTTLS, input.IsDefault,
	)
	cfg, err := scanSMTPConfig(row)
	if err != nil {
		return nil, fmt.Errorf("create smtp config: %w", err)
	}
	return cfg, nil
}

// Update replaces an existing SMTP config. If input.Password is empty, the
// existing password is preserved.
func (r *SMTPConfigRepository) Update(ctx context.Context, id int64, input model.SMTPConfigInput) (*model.SMTPConfig, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE smtp_configs SET
			name         = $1,
			from_email   = NULLIF($2,''),
			host         = $3,
			port         = $4,
			username     = $5,
			password     = CASE WHEN $6 = '' THEN password ELSE $6 END,
			use_tls      = $7,
			use_starttls = $8,
			is_default   = $9,
			updated_at   = now()
		WHERE id = $10
		RETURNING`+smtpConfigColumns,
		input.Name, input.FromEmail, input.Host, input.Port,
		input.Username, input.Password,
		input.UseTLS, input.UseSTARTTLS, input.IsDefault,
		id,
	)
	cfg, err := scanSMTPConfig(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update smtp config: %w", err)
	}
	return cfg, nil
}

// Delete removes the SMTP config with the given id.
func (r *SMTPConfigRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM smtp_configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete smtp config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDefault marks the given config as default and clears the flag on all others,
// atomically in a single transaction.
func (r *SMTPConfigRepository) SetDefault(ctx context.Context, id int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Ensure the target row exists.
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM smtp_configs WHERE id = $1)`, id).Scan(&exists); err != nil {
		return fmt.Errorf("check smtp config existence: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	if _, err := tx.Exec(ctx, `UPDATE smtp_configs SET is_default = false, updated_at = now()`); err != nil {
		return fmt.Errorf("clear default flags: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE smtp_configs SET is_default = true, updated_at = now() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("set default flag: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// scanSMTPConfig scans a row into a SMTPConfig. Works for both pgx.Row and pgx.Rows.
func scanSMTPConfig(row pgx.Row) (*model.SMTPConfig, error) {
	var cfg model.SMTPConfig
	err := row.Scan(
		&cfg.ID,
		&cfg.Name,
		&cfg.FromEmail,
		&cfg.Host,
		&cfg.Port,
		&cfg.Username,
		&cfg.Password,
		&cfg.UseTLS,
		&cfg.UseSTARTTLS,
		&cfg.IsDefault,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

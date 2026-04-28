package users

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("user not found")

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *Repo) GetByEmail(ctx context.Context, q rowQuerier, email string) (*User, string, error) {
	var u User
	var hash string
	err := q.QueryRow(ctx,
		`SELECT id, email, password_hash, full_name, COALESCE(phone,''), is_admin, created_at
		   FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &hash, &u.FullName, &u.Phone, &u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}

func (r *Repo) GetByID(ctx context.Context, q rowQuerier, id string) (*User, error) {
	var u User
	err := q.QueryRow(ctx,
		`SELECT id, email, full_name, COALESCE(phone,''), is_admin, created_at
		   FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Email, &u.FullName, &u.Phone, &u.IsAdmin, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repo) InsertTx(ctx context.Context, tx pgx.Tx, email, hash, fullName, phone string) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, full_name, phone)
		      VALUES ($1, $2, $3, NULLIF($4, ''))
		   RETURNING id`,
		email, hash, fullName, phone).Scan(&id)
	return id, err
}

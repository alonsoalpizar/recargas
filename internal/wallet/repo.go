package wallet

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrWalletNotFound = errors.New("wallet not found")

type Repo struct{}

func NewRepo() *Repo { return &Repo{} }

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *Repo) CreateForUserTx(ctx context.Context, tx pgx.Tx, userID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO wallets (user_id) VALUES ($1) RETURNING id`, userID).Scan(&id)
	return id, err
}

func (r *Repo) GetIDByUser(ctx context.Context, q querier, userID string) (string, error) {
	var id string
	err := q.QueryRow(ctx, `SELECT id FROM wallets WHERE user_id = $1`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrWalletNotFound
	}
	return id, err
}

func (r *Repo) Balance(ctx context.Context, q querier, walletID string) (Balance, error) {
	var b Balance
	err := q.QueryRow(ctx, `
		SELECT
		  COALESCE(SUM(amount_cents) FILTER (WHERE kind = 'real'), 0)::bigint,
		  COALESCE(SUM(amount_cents) FILTER (WHERE kind = 'promo'), 0)::bigint
		FROM wallet_movements
		WHERE wallet_id = $1`, walletID).Scan(&b.RealCents, &b.PromoCents)
	if err != nil {
		return Balance{}, err
	}
	b.TotalCents = b.RealCents + b.PromoCents
	return b, nil
}

type InsertMovement struct {
	WalletID         string
	Type             string
	Kind             string
	AmountCents      int64
	Reason           string
	PerformedByAdmin bool
	PerformedByLabel string
	IdempotencyKey   string
}

func (r *Repo) InsertMovement(ctx context.Context, q querier, m InsertMovement) (*Movement, error) {
	var out Movement
	err := q.QueryRow(ctx, `
		INSERT INTO wallet_movements
		  (wallet_id, type, kind, amount_cents, reason,
		   performed_by_admin, performed_by_label, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
		RETURNING id, wallet_id, type, kind, amount_cents, reason,
		          performed_by_admin, COALESCE(performed_by_label, ''),
		          idempotency_key, created_at`,
		m.WalletID, m.Type, m.Kind, m.AmountCents, m.Reason,
		m.PerformedByAdmin, m.PerformedByLabel, m.IdempotencyKey).
		Scan(&out.ID, &out.WalletID, &out.Type, &out.Kind, &out.AmountCents, &out.Reason,
			&out.PerformedByAdmin, &out.PerformedByLabel,
			&out.IdempotencyKey, &out.CreatedAt)
	return &out, err
}

func (r *Repo) GetByIdempotencyKey(ctx context.Context, q querier, key string) (*Movement, error) {
	var m Movement
	err := q.QueryRow(ctx, `
		SELECT id, wallet_id, type, kind, amount_cents, reason,
		       performed_by_admin, COALESCE(performed_by_label, ''),
		       idempotency_key, created_at
		  FROM wallet_movements WHERE idempotency_key = $1`, key).
		Scan(&m.ID, &m.WalletID, &m.Type, &m.Kind, &m.AmountCents, &m.Reason,
			&m.PerformedByAdmin, &m.PerformedByLabel,
			&m.IdempotencyKey, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

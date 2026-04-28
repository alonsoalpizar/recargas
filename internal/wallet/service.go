package wallet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrUserNotFound  = errors.New("user not found")
)

type Service struct {
	pool *pgxpool.Pool
	repo *Repo
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, repo: NewRepo()}
}

func (s *Service) BalanceForUser(ctx context.Context, userID string) (Balance, error) {
	wid, err := s.repo.GetIDByUser(ctx, s.pool, userID)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			return Balance{}, ErrUserNotFound
		}
		return Balance{}, err
	}
	return s.repo.Balance(ctx, s.pool, wid)
}

type AdjustInput struct {
	UserID         string
	AmountCents    int64
	Kind           string
	Reason         string
	OperatorName   string
	IdempotencyKey string
}

func (s *Service) AdjustManual(ctx context.Context, in AdjustInput) (*Movement, error) {
	if in.AmountCents == 0 {
		return nil, fmt.Errorf("%w: amount_cents must be non-zero", ErrInvalidInput)
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "real"
	}
	if kind != "real" && kind != "promo" {
		return nil, fmt.Errorf("%w: kind must be 'real' or 'promo'", ErrInvalidInput)
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: reason is required", ErrInvalidInput)
	}
	idem := strings.TrimSpace(in.IdempotencyKey)
	if idem == "" {
		return nil, fmt.Errorf("%w: idempotency key is required", ErrInvalidInput)
	}

	if existing, err := s.repo.GetByIdempotencyKey(ctx, s.pool, idem); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	wid, err := s.repo.GetIDByUser(ctx, s.pool, in.UserID)
	if err != nil {
		if errors.Is(err, ErrWalletNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	mv, err := s.repo.InsertMovement(ctx, s.pool, InsertMovement{
		WalletID:         wid,
		Type:             "adjustment",
		Kind:             kind,
		AmountCents:      in.AmountCents,
		Reason:           reason,
		PerformedByAdmin: true,
		PerformedByLabel: strings.TrimSpace(in.OperatorName),
		IdempotencyKey:   idem,
	})
	if err != nil {
		return nil, err
	}

	log.Printf("[EMAIL TO SEND] type=balance_adjustment user_id=%s amount_cents=%d kind=%s reason=%q operator=%q",
		in.UserID, mv.AmountCents, mv.Kind, mv.Reason, mv.PerformedByLabel)
	return mv, nil
}

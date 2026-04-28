package users

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alonsoalpizar/recargas/internal/auth"
	"github.com/alonsoalpizar/recargas/internal/wallet"
)

var (
	ErrEmailTaken     = errors.New("email already registered")
	ErrInvalidInput   = errors.New("invalid input")
	ErrInvalidLogin   = errors.New("invalid email or password")
)

type Service struct {
	pool      *pgxpool.Pool
	repo      *Repo
	wallets   *wallet.Repo
	jwtSecret []byte
}

func NewService(pool *pgxpool.Pool, jwtSecret []byte) *Service {
	return &Service{
		pool:      pool,
		repo:      NewRepo(),
		wallets:   wallet.NewRepo(),
		jwtSecret: jwtSecret,
	}
}

type RegisterInput struct {
	Email    string
	Password string
	FullName string
	Phone    string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	if len(in.Password) < 8 {
		return nil, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	fullName := strings.TrimSpace(in.FullName)
	if fullName == "" {
		return nil, fmt.Errorf("%w: full_name is required", ErrInvalidInput)
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	id, err := s.repo.InsertTx(ctx, tx, email, hash, fullName, strings.TrimSpace(in.Phone))
	if err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	if _, err := s.wallets.CreateForUserTx(ctx, tx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	u, err := s.repo.GetByID(ctx, s.pool, id)
	if err != nil {
		return nil, err
	}

	log.Printf("[EMAIL TO SEND] type=registration to=%s name=%q", u.Email, u.FullName)
	return u, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, *User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, hash, err := s.repo.GetByEmail(ctx, s.pool, email)
	if errors.Is(err, ErrNotFound) {
		return "", nil, ErrInvalidLogin
	}
	if err != nil {
		return "", nil, err
	}
	if !auth.VerifyPassword(hash, password) {
		return "", nil, ErrInvalidLogin
	}
	tok, err := auth.IssueToken(s.jwtSecret, u.ID, u.Email)
	if err != nil {
		return "", nil, err
	}
	return tok, u, nil
}

func (s *Service) Me(ctx context.Context, userID string) (*User, error) {
	return s.repo.GetByID(ctx, s.pool, userID)
}

func isUniqueViolation(err error, constraint string) bool {
	type pgErr interface {
		SQLState() string
	}
	var pe pgErr
	if errors.As(err, &pe) && pe.SQLState() == "23505" {
		return strings.Contains(err.Error(), constraint)
	}
	return false
}

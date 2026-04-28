package users

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alonsoalpizar/recargas/internal/auth"
	"github.com/alonsoalpizar/recargas/internal/services/gometa"
	"github.com/alonsoalpizar/recargas/internal/utils/cedula"
	"github.com/alonsoalpizar/recargas/internal/wallet"
)

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrInvalidCedula  = errors.New("invalid cedula format")
	ErrCedulaTaken    = errors.New("cedula already registered")
	ErrEmailTaken     = errors.New("email already registered")
	ErrPhoneTaken     = errors.New("telefono already registered")
	ErrInvalidLogin   = errors.New("invalid cedula or password")
	ErrCedulaNotFound = errors.New("cedula not found in registro civil")
	ErrGometaOffline  = errors.New("registro civil unavailable, please retry later")
	ErrTOSNotAccepted = errors.New("must accept terms of service")
)

var crPhoneRe = regexp.MustCompile(`^[2-8]\d{7}$`)

type Service struct {
	pool      *pgxpool.Pool
	repo      *Repo
	wallets   *wallet.Repo
	jwtSecret []byte
	gometa    *gometa.Client
}

func NewService(pool *pgxpool.Pool, jwtSecret []byte, g *gometa.Client) *Service {
	return &Service{
		pool:      pool,
		repo:      NewRepo(),
		wallets:   wallet.NewRepo(),
		jwtSecret: jwtSecret,
		gometa:    g,
	}
}

type VerifyResult struct {
	Cedula                string `json:"cedula"`
	Tipo                  string `json:"tipo"`
	Existe                bool   `json:"existe"`
	TienePassword         bool   `json:"tiene_password"`
	ValidadoRegistroCivil bool   `json:"validado_registro_civil"`
	KYCPendingReview      bool   `json:"kyc_pending_review,omitempty"`
	NombreCompleto        string `json:"nombre_completo,omitempty"`
	Nombre                string `json:"nombre,omitempty"`
	Apellido              string `json:"apellido,omitempty"`
}

// VerifyCedula classifies the cedula, checks if it's already registered, and if
// not, validates against GoMeta. Returns enough info for the frontend to decide
// between login form, register form, or DIMEX self-declared form.
func (s *Service) VerifyCedula(ctx context.Context, raw string) (*VerifyResult, error) {
	clean := cedula.Normalize(raw)
	typ := cedula.Detect(clean)
	if typ == cedula.Invalid {
		return nil, ErrInvalidCedula
	}

	if u, err := s.repo.GetByCedula(ctx, s.pool, clean); err == nil {
		return &VerifyResult{
			Cedula:         clean,
			Tipo:           string(typ),
			Existe:         true,
			TienePassword:  true,
			NombreCompleto: u.NombreCompleto,
		}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	lookup := s.gometa.Get(ctx, clean)
	if lookup.Outcome == gometa.Offline {
		return nil, fmt.Errorf("%w: %v", ErrGometaOffline, lookup.Err)
	}

	if lookup.Outcome == gometa.NotFound {
		if typ == cedula.Dimex {
			return &VerifyResult{
				Cedula:                clean,
				Tipo:                  string(typ),
				Existe:                false,
				ValidadoRegistroCivil: false,
				KYCPendingReview:      true,
			}, nil
		}
		return nil, ErrCedulaNotFound
	}

	r := lookup.Response.Results[0]
	return &VerifyResult{
		Cedula:                clean,
		Tipo:                  string(typ),
		Existe:                false,
		ValidadoRegistroCivil: true,
		NombreCompleto:        strings.TrimSpace(lookup.Response.Nombre),
		Nombre:                derefStr(r.Firstname),
		Apellido:              derefStr(r.Lastname),
	}, nil
}

type RegisterInput struct {
	Cedula       string
	Email        string
	Telefono     string
	Password     string
	Nombre       string // self-declared, only used when DIMEX + GoMeta NotFound
	TOSAccepted  bool
	TOSIP        string
	TOSUserAgent string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*User, error) {
	if !in.TOSAccepted {
		return nil, ErrTOSNotAccepted
	}
	if len(in.Password) < 8 {
		return nil, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("%w: invalid email", ErrInvalidInput)
	}
	telefono := normalizeTelefono(in.Telefono)
	if !crPhoneRe.MatchString(telefono) {
		return nil, fmt.Errorf("%w: telefono debe ser 8 dígitos CR (2-8 inicio)", ErrInvalidInput)
	}
	clean := cedula.Normalize(in.Cedula)
	typ := cedula.Detect(clean)
	if typ == cedula.Invalid {
		return nil, ErrInvalidCedula
	}

	lookup := s.gometa.Get(ctx, clean)
	if lookup.Outcome == gometa.Offline {
		return nil, fmt.Errorf("%w: %v", ErrGometaOffline, lookup.Err)
	}

	var nombre, apellido, nombreCompleto string
	var validated, kycPending bool
	var rawGometa map[string]any

	if lookup.Outcome == gometa.Found {
		r := lookup.Response.Results[0]
		nombre = derefStr(r.Firstname)
		apellido = derefStr(r.Lastname)
		nombreCompleto = strings.TrimSpace(lookup.Response.Nombre)
		validated = true
		_ = json.Unmarshal(lookup.RawJSON, &rawGometa)
	} else {
		// NotFound. Only DIMEX is allowed to self-declare.
		if typ != cedula.Dimex {
			return nil, ErrCedulaNotFound
		}
		nombreCompleto = strings.TrimSpace(in.Nombre)
		if nombreCompleto == "" {
			return nil, fmt.Errorf("%w: nombre requerido para DIMEX no validable", ErrInvalidInput)
		}
		kycPending = true
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	metadata := map[string]any{
		"tos": map[string]any{
			"accepted_at": time.Now().UTC().Format(time.RFC3339),
			"ip":          in.TOSIP,
			"user_agent":  in.TOSUserAgent,
			"version":     "v1.0",
		},
		"kyc_pending_review": kycPending,
	}
	if rawGometa != nil {
		metadata["gometa"] = map[string]any{
			"consultado_at": time.Now().UTC().Format(time.RFC3339),
			"validated":     validated,
			"raw":           rawGometa,
		}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	id, err := s.repo.InsertTx(ctx, tx, InsertParams{
		Cedula:         clean,
		CedulaType:     string(typ),
		Nombre:         nombre,
		Apellido:       apellido,
		NombreCompleto: nombreCompleto,
		Email:          email,
		Telefono:       telefono,
		PasswordHash:   hash,
		MetadataJSON:   metadataJSON,
	})
	if err != nil {
		switch {
		case isUniqueViolation(err, "users_cedula_key"):
			return nil, ErrCedulaTaken
		case isUniqueViolation(err, "users_email_key"):
			return nil, ErrEmailTaken
		case isUniqueViolation(err, "users_telefono_key"):
			return nil, ErrPhoneTaken
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

	log.Printf("[EMAIL TO SEND] type=registration to=%s nombre=%q cedula=%s tipo=%s kyc_pending=%t",
		u.Email, u.NombreCompleto, u.Cedula, u.CedulaType, kycPending)
	return u, nil
}

func (s *Service) Login(ctx context.Context, rawCedula, password string) (string, *User, error) {
	clean := cedula.Normalize(rawCedula)
	if !cedula.IsValid(clean) {
		return "", nil, ErrInvalidLogin
	}
	u, hash, err := s.repo.GetByCedulaWithHash(ctx, s.pool, clean)
	if errors.Is(err, ErrNotFound) {
		return "", nil, ErrInvalidLogin
	}
	if err != nil {
		return "", nil, err
	}
	if !auth.VerifyPassword(hash, password) {
		return "", nil, ErrInvalidLogin
	}
	tok, err := auth.IssueToken(s.jwtSecret, u.ID, u.Cedula, u.Email)
	if err != nil {
		return "", nil, err
	}
	return tok, u, nil
}

func (s *Service) Me(ctx context.Context, userID string) (*MeResult, error) {
	u, err := s.repo.GetByID(ctx, s.pool, userID)
	if err != nil {
		return nil, err
	}
	return &MeResult{User: *u, KYCStatus: kycStatus(u)}, nil
}

type UpdateMeInput struct {
	Email           string
	Telefono        string
	Direccion       string
	Provincia       string
	Canton          string
	Distrito        string
	FechaNacimiento string // YYYY-MM-DD or empty (no change)
}

func (s *Service) UpdateMe(ctx context.Context, userID string, in UpdateMeInput) (*MeResult, error) {
	if in.Email != "" {
		email := strings.ToLower(strings.TrimSpace(in.Email))
		if _, err := mail.ParseAddress(email); err != nil {
			return nil, fmt.Errorf("%w: invalid email", ErrInvalidInput)
		}
		in.Email = email
	}
	if in.Telefono != "" {
		t := normalizeTelefono(in.Telefono)
		if !crPhoneRe.MatchString(t) {
			return nil, fmt.Errorf("%w: telefono inválido", ErrInvalidInput)
		}
		in.Telefono = t
	}

	var fechaPtr *string
	if in.FechaNacimiento != "" {
		// Accept YYYY-MM-DD; pass through to PG as date.
		if _, err := time.Parse("2006-01-02", in.FechaNacimiento); err != nil {
			return nil, fmt.Errorf("%w: fecha_nacimiento debe ser YYYY-MM-DD", ErrInvalidInput)
		}
		fechaPtr = &in.FechaNacimiento
	}

	u, err := s.repo.UpdateProfile(ctx, s.pool, UpdateProfileParams{
		UserID:          userID,
		Email:           in.Email,
		Telefono:        in.Telefono,
		Direccion:       in.Direccion,
		Provincia:       in.Provincia,
		Canton:          in.Canton,
		Distrito:        in.Distrito,
		FechaNacimiento: fechaPtr,
	})
	if err != nil {
		switch {
		case isUniqueViolation(err, "users_email_key"):
			return nil, ErrEmailTaken
		case isUniqueViolation(err, "users_telefono_key"):
			return nil, ErrPhoneTaken
		}
		return nil, err
	}
	return &MeResult{User: *u, KYCStatus: kycStatus(u)}, nil
}

func kycStatus(u *User) KYCStatus {
	missing := []string{}
	if u.Direccion == "" {
		missing = append(missing, "direccion")
	}
	if u.Provincia == "" {
		missing = append(missing, "provincia")
	}
	if u.Canton == "" {
		missing = append(missing, "canton")
	}
	if u.Distrito == "" {
		missing = append(missing, "distrito")
	}
	if u.FechaNacimiento == nil {
		missing = append(missing, "fecha_nacimiento")
	}
	return KYCStatus{Complete: len(missing) == 0, Missing: missing}
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func normalizeTelefono(raw string) string {
	digits := regexp.MustCompile(`[^0-9]`).ReplaceAllString(raw, "")
	// Strip leading 506 country code if present.
	digits = strings.TrimPrefix(digits, "506")
	if len(digits) > 8 {
		digits = digits[len(digits)-8:]
	}
	return digits
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

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

const userColumns = `
  id, cedula, cedula_type, COALESCE(nombre,''), COALESCE(apellido,''),
  nombre_completo, email, telefono, is_admin,
  COALESCE(direccion,''), COALESCE(provincia,''), COALESCE(canton,''), COALESCE(distrito,''),
  fecha_nacimiento, created_at
`

func scanUser(row pgx.Row, u *User) error {
	return row.Scan(
		&u.ID, &u.Cedula, &u.CedulaType, &u.Nombre, &u.Apellido,
		&u.NombreCompleto, &u.Email, &u.Telefono, &u.IsAdmin,
		&u.Direccion, &u.Provincia, &u.Canton, &u.Distrito,
		&u.FechaNacimiento, &u.CreatedAt,
	)
}

func (r *Repo) GetByCedula(ctx context.Context, q rowQuerier, cedula string) (*User, error) {
	var u User
	err := scanUser(q.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE cedula = $1`, cedula), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repo) GetByCedulaWithHash(ctx context.Context, q rowQuerier, cedula string) (*User, string, error) {
	var u User
	var hash string
	err := q.QueryRow(ctx, `
		SELECT `+userColumns+`, password_hash
		  FROM users WHERE cedula = $1`, cedula).Scan(
		&u.ID, &u.Cedula, &u.CedulaType, &u.Nombre, &u.Apellido,
		&u.NombreCompleto, &u.Email, &u.Telefono, &u.IsAdmin,
		&u.Direccion, &u.Provincia, &u.Canton, &u.Distrito,
		&u.FechaNacimiento, &u.CreatedAt,
		&hash,
	)
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
	err := scanUser(q.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

type InsertParams struct {
	Cedula         string
	CedulaType     string
	Nombre         string
	Apellido       string
	NombreCompleto string
	Email          string
	Telefono       string
	PasswordHash   string
	MetadataJSON   []byte
}

func (r *Repo) InsertTx(ctx context.Context, tx pgx.Tx, p InsertParams) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO users (
		  cedula, cedula_type, nombre, apellido, nombre_completo,
		  email, telefono, password_hash, metadata
		) VALUES (
		  $1, $2, NULLIF($3,''), NULLIF($4,''), $5,
		  $6, $7, $8, $9::jsonb
		)
		RETURNING id`,
		p.Cedula, p.CedulaType, p.Nombre, p.Apellido, p.NombreCompleto,
		p.Email, p.Telefono, p.PasswordHash, p.MetadataJSON,
	).Scan(&id)
	return id, err
}

type UpdateProfileParams struct {
	UserID          string
	Email           string  // empty = no change
	Telefono        string  // empty = no change
	Direccion       string  // pointer-style: empty string clears, but we treat empty as no-op
	Provincia       string
	Canton          string
	Distrito        string
	FechaNacimiento *string // RFC3339 date or nil
}

// UpdateProfile patches the user. Empty strings are treated as "no change"; pass
// the existing value to keep, or a new value to replace. fecha_nacimiento accepts
// nil for no change or a *string with the new date.
func (r *Repo) UpdateProfile(ctx context.Context, q rowQuerier, p UpdateProfileParams) (*User, error) {
	var u User
	err := scanUser(q.QueryRow(ctx, `
		UPDATE users SET
		  email           = COALESCE(NULLIF($2,''), email),
		  telefono        = COALESCE(NULLIF($3,''), telefono),
		  direccion       = COALESCE(NULLIF($4,''), direccion),
		  provincia       = COALESCE(NULLIF($5,''), provincia),
		  canton          = COALESCE(NULLIF($6,''), canton),
		  distrito        = COALESCE(NULLIF($7,''), distrito),
		  fecha_nacimiento= COALESCE($8::date, fecha_nacimiento),
		  updated_at      = now()
		WHERE id = $1
		RETURNING `+userColumns,
		p.UserID, p.Email, p.Telefono,
		p.Direccion, p.Provincia, p.Canton, p.Distrito,
		p.FechaNacimiento,
	), &u)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

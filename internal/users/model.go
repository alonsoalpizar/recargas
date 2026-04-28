package users

import "time"

type User struct {
	ID              string     `json:"id"`
	Cedula          string     `json:"cedula"`
	CedulaType      string     `json:"cedula_type"`
	Nombre          string     `json:"nombre,omitempty"`
	Apellido        string     `json:"apellido,omitempty"`
	NombreCompleto  string     `json:"nombre_completo"`
	Email           string     `json:"email"`
	Telefono        string     `json:"telefono"`
	IsAdmin         bool       `json:"is_admin"`
	Direccion       string     `json:"direccion,omitempty"`
	Provincia       string     `json:"provincia,omitempty"`
	Canton          string     `json:"canton,omitempty"`
	Distrito        string     `json:"distrito,omitempty"`
	FechaNacimiento *time.Time `json:"fecha_nacimiento,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type KYCStatus struct {
	Complete bool     `json:"complete"`
	Missing  []string `json:"missing"`
}

type MeResult struct {
	User
	KYCStatus KYCStatus `json:"kyc_status"`
}

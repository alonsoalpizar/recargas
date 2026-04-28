package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/alonsoalpizar/recargas/internal/auth"
	"github.com/alonsoalpizar/recargas/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type verifyReq struct {
	Cedula string `json:"cedula"`
}

func (h *Handler) VerifyCedula(w http.ResponseWriter, r *http.Request) {
	var req verifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	res, err := h.svc.VerifyCedula(r.Context(), req.Cedula)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCedula):
			httpx.Error(w, http.StatusBadRequest, "cédula inválida")
		case errors.Is(err, ErrCedulaNotFound):
			httpx.Error(w, http.StatusNotFound, "no pudimos validar esta cédula")
		case errors.Is(err, ErrGometaOffline):
			httpx.Error(w, http.StatusServiceUnavailable, "validación temporalmente no disponible")
		default:
			httpx.Error(w, http.StatusInternalServerError, "no se pudo verificar la cédula")
		}
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

type registerReq struct {
	Cedula      string `json:"cedula"`
	Email       string `json:"email"`
	Telefono    string `json:"telefono"`
	Password    string `json:"password"`
	Nombre      string `json:"nombre"`
	TOSAccepted bool   `json:"tos_accepted"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	u, err := h.svc.Register(r.Context(), RegisterInput{
		Cedula:       req.Cedula,
		Email:        req.Email,
		Telefono:     req.Telefono,
		Password:     req.Password,
		Nombre:       req.Nombre,
		TOSAccepted:  req.TOSAccepted,
		TOSIP:        clientIP(r),
		TOSUserAgent: r.Header.Get("User-Agent"),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrTOSNotAccepted):
			httpx.Error(w, http.StatusBadRequest, "debes aceptar los términos de servicio")
		case errors.Is(err, ErrInvalidCedula):
			httpx.Error(w, http.StatusBadRequest, "cédula inválida")
		case errors.Is(err, ErrCedulaTaken):
			httpx.Error(w, http.StatusConflict, "esta cédula ya tiene cuenta registrada")
		case errors.Is(err, ErrEmailTaken):
			httpx.Error(w, http.StatusConflict, "el email ya está registrado")
		case errors.Is(err, ErrPhoneTaken):
			httpx.Error(w, http.StatusConflict, "el teléfono ya está registrado")
		case errors.Is(err, ErrCedulaNotFound):
			httpx.Error(w, http.StatusNotFound, "no pudimos validar esta cédula")
		case errors.Is(err, ErrGometaOffline):
			httpx.Error(w, http.StatusServiceUnavailable, "validación temporalmente no disponible")
		case errors.Is(err, ErrInvalidInput):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, "no se pudo registrar")
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, u)
}

type loginReq struct {
	Cedula   string `json:"cedula"`
	Password string `json:"password"`
}

type loginResp struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	tok, u, err := h.svc.Login(r.Context(), req.Cedula, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidLogin) {
			httpx.Error(w, http.StatusUnauthorized, "cédula o password incorrectos")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "no se pudo iniciar sesión")
		return
	}
	httpx.JSON(w, http.StatusOK, loginResp{Token: tok, User: u})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "no user in context")
		return
	}
	res, err := h.svc.Me(r.Context(), uid)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "user not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "no se pudo cargar el perfil")
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

type updateMeReq struct {
	Email           string `json:"email,omitempty"`
	Telefono        string `json:"telefono,omitempty"`
	Direccion       string `json:"direccion,omitempty"`
	Provincia       string `json:"provincia,omitempty"`
	Canton          string `json:"canton,omitempty"`
	Distrito        string `json:"distrito,omitempty"`
	FechaNacimiento string `json:"fecha_nacimiento,omitempty"`
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "no user in context")
		return
	}
	var req updateMeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	res, err := h.svc.UpdateMe(r.Context(), uid, UpdateMeInput{
		Email:           req.Email,
		Telefono:        req.Telefono,
		Direccion:       req.Direccion,
		Provincia:       req.Provincia,
		Canton:          req.Canton,
		Distrito:        req.Distrito,
		FechaNacimiento: req.FechaNacimiento,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrEmailTaken):
			httpx.Error(w, http.StatusConflict, "el email ya está registrado por otro usuario")
		case errors.Is(err, ErrPhoneTaken):
			httpx.Error(w, http.StatusConflict, "el teléfono ya está registrado por otro usuario")
		default:
			httpx.Error(w, http.StatusInternalServerError, "no se pudo actualizar el perfil")
		}
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in the chain is the real client (real_ip already trimmed CF).
		if i := strings.Index(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return rip
	}
	return r.RemoteAddr
}

package wallet

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/alonsoalpizar/recargas/internal/auth"
	"github.com/alonsoalpizar/recargas/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Balance(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "no user in context")
		return
	}
	b, err := h.svc.BalanceForUser(r.Context(), uid)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			httpx.Error(w, http.StatusNotFound, "wallet not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "could not read balance")
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

type adjustReq struct {
	AmountCents  int64  `json:"amount_cents"`
	Kind         string `json:"kind"`
	Reason       string `json:"reason"`
	OperatorName string `json:"operator_name"`
}

func (h *Handler) Adjust(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		httpx.Error(w, http.StatusBadRequest, "missing userId")
		return
	}
	idem := r.Header.Get("Idempotency-Key")
	if idem == "" {
		httpx.Error(w, http.StatusBadRequest, "missing Idempotency-Key header")
		return
	}
	var req adjustReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid json body")
		return
	}

	mv, err := h.svc.AdjustManual(r.Context(), AdjustInput{
		UserID:         userID,
		AmountCents:    req.AmountCents,
		Kind:           req.Kind,
		Reason:         req.Reason,
		OperatorName:   req.OperatorName,
		IdempotencyKey: idem,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrUserNotFound):
			httpx.Error(w, http.StatusNotFound, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, "could not adjust wallet")
		}
		return
	}

	httpx.JSON(w, http.StatusCreated, mv)
}

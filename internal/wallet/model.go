package wallet

import "time"

type Balance struct {
	RealCents  int64 `json:"real_cents"`
	PromoCents int64 `json:"promo_cents"`
	TotalCents int64 `json:"total_cents"`
}

type Movement struct {
	ID                 string    `json:"id"`
	WalletID           string    `json:"wallet_id"`
	Type               string    `json:"type"`
	Kind               string    `json:"kind"`
	AmountCents        int64     `json:"amount_cents"`
	Reason             string    `json:"reason"`
	PerformedByAdmin   bool      `json:"performed_by_admin"`
	PerformedByLabel   string    `json:"performed_by_label,omitempty"`
	IdempotencyKey     string    `json:"idempotency_key"`
	CreatedAt          time.Time `json:"created_at"`
}

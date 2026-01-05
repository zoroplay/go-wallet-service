package models

type Wallet struct {
	ClientID            string  `json:"client_id"`
	UserID              string  `json:"user_id"`
	Username            string  `json:"username"`
	Balance             float64 `json:"balance"`
	AvailableBalance    float64 `json:"available_balance"`
	TrustBalance        float64 `json:"trust_balance"`
	SportBonusBalance   float64 `json:"sport_bonus_balance"`
	VirtualBonusBalance float64 `json:"virtual_bonus_balance"`
	CasinoBonusBalance  float64 `json:"casino_bonus_balance"`
	Status              int64   `json:"status"`
}

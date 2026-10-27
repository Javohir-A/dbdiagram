package dto

import "gitlab.com/udevs/dbdiagram/backend/internal/model"

type CreateTokenRequest struct {
	Name  string           `json:"name" binding:"required"`
	Scope model.TokenScope `json:"scope" binding:"required"`
	// ExpiresInDays is 1..365; omitted means the default (90).
	ExpiresInDays int `json:"expires_in_days"`
}

// TokenList documents model.List[*model.AccessToken] for swagger.
type TokenList struct {
	Count int32               `json:"count"`
	Items []model.AccessToken `json:"items"`
}

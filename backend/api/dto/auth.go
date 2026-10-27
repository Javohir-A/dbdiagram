package dto

import "gitlab.com/udevs/dbdiagram/backend/internal/model"

type RegisterRequest struct {
	Email    string `json:"email" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type UpdateMeRequest struct {
	Name string `json:"name" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type SetMemberRequest struct {
	Email string     `json:"email" binding:"required"`
	Role  model.Role `json:"role" binding:"required"`
}

type SetParentRequest struct {
	// ParentID is the new parent folder guid; "" makes the folder top level.
	ParentID string `json:"parent_id"`
}

// MemberList documents []*model.Member for swagger.
type MemberList struct {
	Count int32          `json:"count"`
	Items []model.Member `json:"items"`
}

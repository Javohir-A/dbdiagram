package handler

import (
	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/dto"
	"gitlab.com/udevs/dbdiagram/backend/api/middleware"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/core"
)

// @Router /v1/auth/register [post]
// @Summary Register
// @Description Creates an account and signs it in. Rate limited per IP.
// @Tags auth
// @Accept  json
// @Produce  json
// @Param body body dto.RegisterRequest true "account"
// @Success 200 {object} model.AuthResult
// @Failure 400 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 409 {object} dto.Error
// @Failure 429 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) Register(c *gin.Context) {
	var body dto.RegisterRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Core.Auth.Register(c.Request.Context(), core.RegisterParams{
		Email: body.Email, Name: body.Name, Password: body.Password, UserAgent: c.Request.UserAgent(),
	})

	h.respond(c, res, err)
}

// @Router /v1/auth/login [post]
// @Summary Login
// @Description Signs in with email and password. A wrong email and a wrong password get the same answer.
// @Tags auth
// @Accept  json
// @Produce  json
// @Param body body dto.LoginRequest true "credentials"
// @Success 200 {object} model.AuthResult
// @Failure 400 {object} dto.Error
// @Failure 401 {object} dto.Error
// @Failure 429 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) Login(c *gin.Context) {
	var body dto.LoginRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Core.Auth.Login(c.Request.Context(), core.LoginParams{
		Email: body.Email, Password: body.Password, UserAgent: c.Request.UserAgent(),
	})

	h.respond(c, res, err)
}

// @Router /v1/auth/refresh [post]
// @Summary Refresh tokens
// @Description Trades a refresh token for a new access and refresh token. The old refresh token stops working; presenting one that was already used ends the whole login.
// @Tags auth
// @Accept  json
// @Produce  json
// @Param body body dto.RefreshRequest true "refresh token"
// @Success 200 {object} model.AuthResult
// @Failure 400 {object} dto.Error
// @Failure 401 {object} dto.Error
// @Failure 429 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) Refresh(c *gin.Context) {
	var body dto.RefreshRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Core.Auth.Refresh(c.Request.Context(), body.RefreshToken, c.Request.UserAgent())

	h.respond(c, res, err)
}

// @Router /v1/auth/logout [post]
// @Summary Logout
// @Description Ends the login the refresh token belongs to. Always succeeds for an unknown token.
// @Tags auth
// @Accept  json
// @Produce  json
// @Param body body dto.RefreshRequest true "refresh token"
// @Success 200 {object} dto.Empty
// @Failure 400 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) Logout(c *gin.Context) {
	var body dto.RefreshRequest
	if !h.bindJSON(c, &body) {
		return
	}

	err := h.app.Core.Auth.Logout(c.Request.Context(), body.RefreshToken)

	h.respondOK(c, err)
}

// @Router /v1/me [get]
// @Security ApiKeyAuth
// @Summary Current user
// @Tags auth
// @Produce  json
// @Success 200 {object} model.User
// @Failure 401 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) GetMe(c *gin.Context) {
	res, err := h.app.Core.User.Get(c.Request.Context(), middleware.ScopeOf(c).UserID)

	h.respond(c, res, err)
}

// @Router /v1/me [patch]
// @Security ApiKeyAuth
// @Summary Update profile
// @Tags auth
// @Accept  json
// @Produce  json
// @Param body body dto.UpdateMeRequest true "profile"
// @Success 200 {object} model.User
// @Failure 400 {object} dto.Error
// @Failure 401 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) UpdateMe(c *gin.Context) {
	var body dto.UpdateMeRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Core.User.UpdateName(c.Request.Context(), middleware.ScopeOf(c).UserID, body.Name)

	h.respond(c, res, err)
}

// @Router /v1/me/password [put]
// @Security ApiKeyAuth
// @Summary Change password
// @Description Needs the current password. Every other login of the account is ended.
// @Tags auth
// @Accept  json
// @Produce  json
// @Param body body dto.ChangePasswordRequest true "passwords"
// @Success 200 {object} dto.Empty
// @Failure 400 {object} dto.Error
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) ChangePassword(c *gin.Context) {
	var body dto.ChangePasswordRequest
	if !h.bindJSON(c, &body) {
		return
	}

	scope := middleware.ScopeOf(c)
	err := h.app.Core.User.ChangePassword(c.Request.Context(), scope.UserID, body.CurrentPassword, body.NewPassword, scope.Family)

	h.respondOK(c, err)
}

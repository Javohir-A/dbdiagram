package handler

import (
	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/dto"
	"gitlab.com/udevs/dbdiagram/backend/api/middleware"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/core"
)

// @Router /v1/me/tokens [get]
// @Security ApiKeyAuth
// @Summary Personal access tokens
// @Description Lists your active (not revoked, not expired) tokens. Needs a login session, not a token.
// @Tags tokens
// @Produce  json
// @Success 200 {object} dto.TokenList
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) ListTokens(c *gin.Context) {
	res, err := h.app.Core.Token.List(c.Request.Context(), middleware.ScopeOf(c).UserID)

	h.respond(c, res, err)
}

// @Router /v1/me/tokens [post]
// @Security ApiKeyAuth
// @Summary Create a personal access token
// @Description Creates a token for scripts and AI agents (including the MCP endpoint). The secret is in the response once and is never shown again. Needs a login session, not a token.
// @Tags tokens
// @Accept  json
// @Produce  json
// @Param body body dto.CreateTokenRequest true "token"
// @Success 200 {object} model.CreatedAccessToken
// @Failure 400 {object} dto.Error
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) CreateToken(c *gin.Context) {
	var body dto.CreateTokenRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Core.Token.Create(c.Request.Context(), middleware.ScopeOf(c).UserID, core.CreateTokenParams{
		Name: body.Name, Scope: body.Scope, ExpiresInDays: body.ExpiresInDays,
	})

	h.respond(c, res, err)
}

// @Router /v1/me/tokens/{key} [delete]
// @Security ApiKeyAuth
// @Summary Revoke a personal access token
// @Description Takes effect immediately. Needs a login session, not a token.
// @Tags tokens
// @Produce  json
// @Param key path string true "token guid"
// @Success 200 {object} dto.Empty
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) RevokeToken(c *gin.Context) {
	err := h.app.Core.Token.Revoke(c.Request.Context(), middleware.ScopeOf(c).UserID, c.Param("key"))

	h.respondOK(c, err)
}

package handler

import (
	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/dto"
)

// @Router /v1/health [get]
// @Summary Health
// @Description Answers ok while the database answers.
// @Tags system
// @Produce  json
// @Success 200 {object} dto.Health
// @Failure 503 {object} dto.Error
func (h *Handler) Health(c *gin.Context) {
	if err := h.app.Ping(c.Request.Context()); err != nil {
		h.respond(c, nil, err)
		return
	}

	h.respond(c, dto.Health{Status: "ok"}, nil)
}

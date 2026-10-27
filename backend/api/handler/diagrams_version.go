package handler

import (
	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/dto"
	"gitlab.com/udevs/dbdiagram/backend/api/middleware"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
)

// @Router /v1/diagrams/{key}/versions [get]
// @Security ApiKeyAuth
// @Summary Diagram version list
// @Description Snapshots of the diagram, newest first, without their DBML. An empty label marks an automatic snapshot.
// @Tags versions
// @Produce  json
// @Param key path string true "diagram guid"
// @Param page query int false "page, from 1" default(1)
// @Param limit query int false "page size, max 1000" default(10)
// @Success 200 {object} dto.VersionList
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) ListDiagramVersions(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	page, err := ParsePageQueryParam(c)
	if err != nil {
		h.badRequest(c, "invalid page")
		return
	}
	limit, err := ParseLimitQueryParam(c)
	if err != nil {
		h.badRequest(c, "invalid limit")
		return
	}

	res, err := h.app.Diagrams.Version.Find(c.Request.Context(), actor, diagrams.FindVersionsParams{
		DiagramGuid: c.Param("key"), Page: page, Limit: limit,
	})

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key}/versions [post]
// @Security ApiKeyAuth
// @Summary Diagram version create
// @Description Takes a manual snapshot of the saved state of the diagram. The body is optional.
// @Tags versions
// @Accept  json
// @Produce  json
// @Param key path string true "diagram guid"
// @Param body body dto.CreateVersionRequest false "label"
// @Success 200 {object} model.VersionItem
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) CreateDiagramVersion(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	var body dto.CreateVersionRequest
	// A bare POST (no body at all) is a valid unlabelled snapshot.
	if c.Request.ContentLength != 0 && !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Version.Create(c.Request.Context(), actor, c.Param("key"), body.Label)

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key}/versions/{version} [get]
// @Security ApiKeyAuth
// @Summary Diagram version get
// @Description Returns one snapshot with its DBML and layout. A version of another diagram is not found.
// @Tags versions
// @Produce  json
// @Param key path string true "diagram guid"
// @Param version path string true "version guid"
// @Success 200 {object} model.Version
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) GetDiagramVersion(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	res, err := h.app.Diagrams.Version.Get(c.Request.Context(), actor, c.Param("key"), c.Param("version"))

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key}/versions/{version}/restore [post]
// @Security ApiKeyAuth
// @Summary Diagram version restore
// @Description Replaces the diagram's DBML and layout with the snapshot's; the name is kept. The current state is saved as a new snapshot first, so a restore can be undone.
// @Tags versions
// @Produce  json
// @Param key path string true "diagram guid"
// @Param version path string true "version guid"
// @Success 200 {object} model.Diagram
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) RestoreDiagramVersion(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	res, err := h.app.Diagrams.Version.Restore(c.Request.Context(), actor, c.Param("key"), c.Param("version"))

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key}/versions/{version} [delete]
// @Security ApiKeyAuth
// @Summary Diagram version delete
// @Tags versions
// @Produce  json
// @Param key path string true "diagram guid"
// @Param version path string true "version guid"
// @Success 200 {object} dto.Empty
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) DeleteDiagramVersion(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	err := h.app.Diagrams.Version.Delete(c.Request.Context(), actor, c.Param("key"), c.Param("version"))

	h.respondOK(c, err)
}

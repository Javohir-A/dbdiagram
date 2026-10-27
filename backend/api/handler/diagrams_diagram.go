package handler

import (
	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/dto"
	"gitlab.com/udevs/dbdiagram/backend/api/middleware"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
)

// @Router /v1/diagrams [post]
// @Security ApiKeyAuth
// @Summary Diagram create
// @Description Creates a diagram. dbml and positions may be omitted for an empty one.
// @Tags diagrams
// @Accept  json
// @Produce  json
// @Param body body dto.CreateDiagramRequest true "diagram"
// @Success 200 {object} model.Diagram
// @Failure 400 {object} dto.Error
// @Failure 413 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) CreateDiagram(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	var body dto.CreateDiagramRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Diagram.Create(c.Request.Context(), actor, diagrams.CreateDiagramParams{
		Name: body.Name, DBML: body.DBML, Positions: body.Positions, FolderID: body.FolderID,
	})

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key} [get]
// @Security ApiKeyAuth
// @Summary Diagram get
// @Description Returns one diagram with its DBML and layout.
// @Tags diagrams
// @Produce  json
// @Param key path string true "diagram guid"
// @Success 200 {object} model.Diagram
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) GetDiagram(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	res, err := h.app.Diagrams.Diagram.Get(c.Request.Context(), actor, c.Param("key"))

	h.respond(c, res, err)
}

// @Router /v1/diagrams [get]
// @Security ApiKeyAuth
// @Summary Diagram list
// @Description Lists diagrams without their DBML. search matches the name; filters is a JSON array [{field,type,value}] over name, folder_id (empty value = unfiled), created_at, updated_at; sort is a JSON array [{field,type}] over name, created_at, updated_at (default updated_at desc).
// @Tags diagrams
// @Produce  json
// @Param search query string false "text in the name"
// @Param filters query string false "JSON array of {field,type,value}"
// @Param sort query string false "JSON array of {field,type}"
// @Param page query int false "page, from 1" default(1)
// @Param limit query int false "page size, max 1000" default(10)
// @Success 200 {object} dto.DiagramList
// @Failure 400 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) ListDiagrams(c *gin.Context) {
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
	filters, err := ParseFilters(c)
	if err != nil {
		h.badRequest(c, "invalid filters")
		return
	}
	sort, err := ParseSort(c)
	if err != nil {
		h.badRequest(c, "invalid sort")
		return
	}

	res, err := h.app.Diagrams.Diagram.Find(c.Request.Context(), actor, diagrams.FindDiagramsParams{
		Search: c.Query("search"), Filters: filters, Sort: sort, Page: page, Limit: limit,
	})

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key} [put]
// @Security ApiKeyAuth
// @Summary Diagram replace
// @Description Replaces the whole mutable representation; an omitted dbml or positions becomes empty. Use PATCH to change only some fields.
// @Tags diagrams
// @Accept  json
// @Produce  json
// @Param key path string true "diagram guid"
// @Param body body dto.ReplaceDiagramRequest true "diagram"
// @Success 200 {object} model.Diagram
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 413 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) ReplaceDiagram(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	var body dto.ReplaceDiagramRequest
	if !h.bindJSON(c, &body) {
		return
	}
	if body.Positions == nil {
		body.Positions = map[string]model.Position{}
	}

	res, err := h.app.Diagrams.Diagram.Update(c.Request.Context(), actor, diagrams.UpdateDiagramParams{
		Guid: c.Param("key"), Name: &body.Name, DBML: &body.DBML, Positions: &body.Positions,
		BaseRevision: body.BaseRevision,
	})

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key} [patch]
// @Security ApiKeyAuth
// @Summary Diagram update
// @Description Changes only the fields that are sent; a field left out keeps its value.
// @Tags diagrams
// @Accept  json
// @Produce  json
// @Param key path string true "diagram guid"
// @Param body body dto.PatchDiagramRequest true "fields to change"
// @Success 200 {object} model.Diagram
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 413 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) PatchDiagram(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	var body dto.PatchDiagramRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Diagram.Update(c.Request.Context(), actor, diagrams.UpdateDiagramParams{
		Guid: c.Param("key"), Name: body.Name, DBML: body.DBML, Positions: body.Positions, BaseRevision: body.BaseRevision,
	})

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key}/folder [put]
// @Security ApiKeyAuth
// @Summary Diagram move to folder
// @Description Files the diagram in a folder; an empty folder_id takes it out of its folder.
// @Tags diagrams
// @Accept  json
// @Produce  json
// @Param key path string true "diagram guid"
// @Param body body dto.SetDiagramFolderRequest true "folder"
// @Success 200 {object} model.Diagram
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) SetDiagramFolder(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	var body dto.SetDiagramFolderRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Diagram.SetFolder(c.Request.Context(), actor, c.Param("key"), body.FolderID)

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key}/share [put]
// @Security ApiKeyAuth
// @Summary Diagram share
// @Description Turns the public read-only link on or off. Turning it on again keeps the same token.
// @Tags diagrams
// @Accept  json
// @Produce  json
// @Param key path string true "diagram guid"
// @Param body body dto.SetDiagramShareRequest true "share switch"
// @Success 200 {object} model.Diagram
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) SetDiagramShare(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	var body dto.SetDiagramShareRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Diagram.SetShare(c.Request.Context(), actor, c.Param("key"), *body.Enabled)

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key} [delete]
// @Security ApiKeyAuth
// @Summary Diagram delete
// @Description Soft deletes the diagram; its public link stops working at once.
// @Tags diagrams
// @Produce  json
// @Param key path string true "diagram guid"
// @Success 200 {object} dto.Empty
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) DeleteDiagram(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	err := h.app.Diagrams.Diagram.Delete(c.Request.Context(), actor, c.Param("key"))

	h.respondOK(c, err)
}

// @Router /v1/shared/{key} [get]
// @Summary Shared diagram get
// @Description Public and read-only: returns the name, DBML and layout behind a share token, nothing else.
// @Tags shared
// @Produce  json
// @Param key path string true "share token"
// @Success 200 {object} model.SharedDiagram
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) GetSharedDiagram(c *gin.Context) {
	res, err := h.app.Diagrams.Diagram.GetShared(c.Request.Context(), c.Param("key"))

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key}/revision [get]
// @Security ApiKeyAuth
// @Summary Diagram revision
// @Description The tiny change marker the editor polls: the DBML revision and who last saved.
// @Tags diagrams
// @Produce  json
// @Param key path string true "diagram guid"
// @Success 200 {object} model.Revision
// @Failure 400 {object} dto.Error
// @Failure 401 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) GetDiagramRevision(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID

	res, err := h.app.Diagrams.Diagram.Revision(c.Request.Context(), actor, c.Param("key"))

	h.respond(c, res, err)
}

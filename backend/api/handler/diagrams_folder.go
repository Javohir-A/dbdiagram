package handler

import (
	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/dto"
	"gitlab.com/udevs/dbdiagram/backend/api/middleware"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
)

// @Router /v1/folders [post]
// @Security ApiKeyAuth
// @Summary Folder create
// @Tags folders
// @Accept  json
// @Produce  json
// @Param body body dto.CreateFolderRequest true "folder"
// @Success 200 {object} model.Folder
// @Failure 400 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) CreateFolder(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	var body dto.CreateFolderRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Folder.Create(c.Request.Context(), actor, body.Name, body.ParentID)

	h.respond(c, res, err)
}

// @Router /v1/folders/{key} [get]
// @Security ApiKeyAuth
// @Summary Folder get
// @Tags folders
// @Produce  json
// @Param key path string true "folder guid"
// @Success 200 {object} model.Folder
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) GetFolder(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	res, err := h.app.Diagrams.Folder.Get(c.Request.Context(), actor, c.Param("key"))

	h.respond(c, res, err)
}

// @Router /v1/folders [get]
// @Security ApiKeyAuth
// @Summary Folder list
// @Description search matches the name; sort is a JSON array [{field,type}] over name, created_at, updated_at (default name asc).
// @Tags folders
// @Produce  json
// @Param search query string false "text in the name"
// @Param sort query string false "JSON array of {field,type}"
// @Param page query int false "page, from 1" default(1)
// @Param limit query int false "page size, max 1000" default(10)
// @Success 200 {object} dto.FolderList
// @Failure 400 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) ListFolders(c *gin.Context) {
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
	sort, err := ParseSort(c)
	if err != nil {
		h.badRequest(c, "invalid sort")
		return
	}

	res, err := h.app.Diagrams.Folder.Find(c.Request.Context(), actor, diagrams.FindFoldersParams{
		Search: c.Query("search"), Sort: sort, Page: page, Limit: limit,
	})

	h.respond(c, res, err)
}

// @Router /v1/folders/{key} [put]
// @Security ApiKeyAuth
// @Summary Folder replace
// @Tags folders
// @Accept  json
// @Produce  json
// @Param key path string true "folder guid"
// @Param body body dto.ReplaceFolderRequest true "folder"
// @Success 200 {object} model.Folder
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) ReplaceFolder(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	var body dto.ReplaceFolderRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Folder.Update(c.Request.Context(), actor, c.Param("key"), &body.Name)

	h.respond(c, res, err)
}

// @Router /v1/folders/{key} [patch]
// @Security ApiKeyAuth
// @Summary Folder update
// @Description Changes only the fields that are sent.
// @Tags folders
// @Accept  json
// @Produce  json
// @Param key path string true "folder guid"
// @Param body body dto.PatchFolderRequest true "fields to change"
// @Success 200 {object} model.Folder
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) PatchFolder(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	var body dto.PatchFolderRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Folder.Update(c.Request.Context(), actor, c.Param("key"), body.Name)

	h.respond(c, res, err)
}

// @Router /v1/folders/{key} [delete]
// @Security ApiKeyAuth
// @Summary Folder delete
// @Description Deletes an empty folder (owner only). A folder that still holds folders or diagrams answers 409.
// @Tags folders
// @Produce  json
// @Param key path string true "folder guid"
// @Success 200 {object} dto.Empty
// @Failure 400 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) DeleteFolder(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID
	err := h.app.Diagrams.Folder.Delete(c.Request.Context(), actor, c.Param("key"))

	h.respondOK(c, err)
}

// @Router /v1/folders/{key}/parent [put]
// @Security ApiKeyAuth
// @Summary Folder move
// @Description Moves the folder under another one (or to the top with an empty parent_id). Needs owner on the folder and editor on the destination; a folder cannot go into itself or its own subtree.
// @Tags folders
// @Accept  json
// @Produce  json
// @Param key path string true "folder guid"
// @Param body body dto.SetParentRequest true "new parent"
// @Success 200 {object} model.Folder
// @Failure 400 {object} dto.Error
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) SetFolderParent(c *gin.Context) {
	actor := middleware.ScopeOf(c).UserID

	var body dto.SetParentRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Folder.SetParent(c.Request.Context(), actor, c.Param("key"), body.ParentID)

	h.respond(c, res, err)
}

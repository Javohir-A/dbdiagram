package handler

import (
	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/api/dto"
	"gitlab.com/udevs/dbdiagram/backend/api/middleware"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
)

func memberList(items []*model.Member, err error) (*model.List[*model.Member], error) {
	if err != nil {
		return nil, err
	}

	return &model.List[*model.Member]{Items: items, Count: int32(len(items))}, nil
}

// @Router /v1/folders/{key}/members [get]
// @Security ApiKeyAuth
// @Summary Folder members
// @Description Who the folder is shared with: the owner first, then direct shares. Owner only. Access inherited from an ancestor folder is managed on that folder.
// @Tags members
// @Produce  json
// @Param key path string true "folder guid"
// @Success 200 {object} dto.MemberList
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) ListFolderMembers(c *gin.Context) {
	res, err := memberList(h.app.Diagrams.Member.ListFolder(c.Request.Context(), middleware.ScopeOf(c).UserID, c.Param("key")))

	h.respond(c, res, err)
}

// @Router /v1/folders/{key}/members [put]
// @Security ApiKeyAuth
// @Summary Share a folder
// @Description Gives a person (by email) editor or viewer access to the folder and everything inside it; calling it again changes their role. Owner only.
// @Tags members
// @Accept  json
// @Produce  json
// @Param key path string true "folder guid"
// @Param body body dto.SetMemberRequest true "person and role"
// @Success 200 {object} model.Member
// @Failure 400 {object} dto.Error
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) SetFolderMember(c *gin.Context) {
	var body dto.SetMemberRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Member.SetFolder(c.Request.Context(), middleware.ScopeOf(c).UserID, c.Param("key"), body.Email, body.Role)

	h.respond(c, res, err)
}

// @Router /v1/folders/{key}/members/{user} [delete]
// @Security ApiKeyAuth
// @Summary Unshare a folder
// @Description Removes a direct share. The owner can remove anyone; a member can remove themselves (leave).
// @Tags members
// @Produce  json
// @Param key path string true "folder guid"
// @Param user path string true "user guid"
// @Success 200 {object} dto.Empty
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) RemoveFolderMember(c *gin.Context) {
	err := h.app.Diagrams.Member.RemoveFolder(c.Request.Context(), middleware.ScopeOf(c).UserID, c.Param("key"), c.Param("user"))

	h.respondOK(c, err)
}

// @Router /v1/diagrams/{key}/members [get]
// @Security ApiKeyAuth
// @Summary Diagram members
// @Description Who the diagram is shared with directly: the owner first, then direct shares. Owner only.
// @Tags members
// @Produce  json
// @Param key path string true "diagram guid"
// @Success 200 {object} dto.MemberList
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) ListDiagramMembers(c *gin.Context) {
	res, err := memberList(h.app.Diagrams.Member.ListDiagram(c.Request.Context(), middleware.ScopeOf(c).UserID, c.Param("key")))

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key}/members [put]
// @Security ApiKeyAuth
// @Summary Share a diagram
// @Description Gives a person (by email) editor or viewer access to this diagram; calling it again changes their role. Owner only.
// @Tags members
// @Accept  json
// @Produce  json
// @Param key path string true "diagram guid"
// @Param body body dto.SetMemberRequest true "person and role"
// @Success 200 {object} model.Member
// @Failure 400 {object} dto.Error
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) SetDiagramMember(c *gin.Context) {
	var body dto.SetMemberRequest
	if !h.bindJSON(c, &body) {
		return
	}

	res, err := h.app.Diagrams.Member.SetDiagram(c.Request.Context(), middleware.ScopeOf(c).UserID, c.Param("key"), body.Email, body.Role)

	h.respond(c, res, err)
}

// @Router /v1/diagrams/{key}/members/{user} [delete]
// @Security ApiKeyAuth
// @Summary Unshare a diagram
// @Description Removes a direct share. The owner can remove anyone; a member can remove themselves (leave).
// @Tags members
// @Produce  json
// @Param key path string true "diagram guid"
// @Param user path string true "user guid"
// @Success 200 {object} dto.Empty
// @Failure 401 {object} dto.Error
// @Failure 403 {object} dto.Error
// @Failure 404 {object} dto.Error
// @Failure 500 {object} dto.Error
func (h *Handler) RemoveDiagramMember(c *gin.Context) {
	err := h.app.Diagrams.Member.RemoveDiagram(c.Request.Context(), middleware.ScopeOf(c).UserID, c.Param("key"), c.Param("user"))

	h.respondOK(c, err)
}

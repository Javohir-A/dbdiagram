package mcp

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/internal/model"
)

// resourceArg is how every sharing tool names its target.
type resourceArg struct {
	Resource string `json:"resource"`
	ID       string `json:"id"`
}

func (r resourceArg) check() error {
	if r.Resource != "diagram" && r.Resource != "folder" {
		return badInput("resource must be \"diagram\" or \"folder\"")
	}
	if r.ID == "" {
		return badInput("id is required")
	}

	return nil
}

var resourceProps = props{
	"resource": oneOf("What is shared.", "diagram", "folder"),
	"id":       str("The diagram's or folder's id."),
}

func with(extra props) props { return with2(resourceProps, extra) }

func (s *Server) sharingTools() []*tool {
	return []*tool{
		{
			Name:        "list_members",
			Description: "List who has direct access to a diagram or folder: the owner first, then people it is shared with and their role. Owner only. A folder's share also covers everything inside it.",
			Schema:      object(resourceProps, "resource", "id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[resourceArg](raw)
				if err != nil {
					return nil, err
				}
				if err := a.check(); err != nil {
					return nil, err
				}

				var members []*model.Member
				if a.Resource == "folder" {
					members, err = s.app.Diagrams.Member.ListFolder(c.Request.Context(), who.UserID, a.ID)
				} else {
					members, err = s.app.Diagrams.Member.ListDiagram(c.Request.Context(), who.UserID, a.ID)
				}
				if err != nil {
					return nil, err
				}

				return map[string]any{"members": members}, nil
			},
		},
		{
			Name:        "share",
			Description: "Give a person access to a diagram or folder by their account email, as editor (can edit) or viewer (read only); calling it again changes their role. Owner only. Sharing a folder gives access to everything inside it.",
			Write:       true,
			Schema:      object(with(props{"email": str("Email of an existing account."), "role": oneOf("Access level.", "editor", "viewer")}), "resource", "id", "email", "role"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					resourceArg
					Email string     `json:"email"`
					Role  model.Role `json:"role"`
				}](raw)
				if err != nil {
					return nil, err
				}
				if err := a.check(); err != nil {
					return nil, err
				}

				var m *model.Member
				if a.Resource == "folder" {
					m, err = s.app.Diagrams.Member.SetFolder(c.Request.Context(), who.UserID, a.ID, a.Email, a.Role)
				} else {
					m, err = s.app.Diagrams.Member.SetDiagram(c.Request.Context(), who.UserID, a.ID, a.Email, a.Role)
				}
				if err != nil {
					return nil, err
				}

				return m, nil
			},
		},
		{
			Name:        "unshare",
			Description: "Remove a person's direct access to a diagram or folder (owner only; anyone may remove themselves). Use list_members to find the user_id.",
			Write:       true,
			Destructive: true,
			Schema:      object(with(props{"user_id": str("The person's user id from list_members.")}), "resource", "id", "user_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					resourceArg
					UserID string `json:"user_id"`
				}](raw)
				if err != nil {
					return nil, err
				}
				if err := a.check(); err != nil {
					return nil, err
				}

				if a.Resource == "folder" {
					err = s.app.Diagrams.Member.RemoveFolder(c.Request.Context(), who.UserID, a.ID, a.UserID)
				} else {
					err = s.app.Diagrams.Member.RemoveDiagram(c.Request.Context(), who.UserID, a.ID, a.UserID)
				}
				if err != nil {
					return nil, err
				}

				return map[string]any{"removed": a.UserID}, nil
			},
		},
	}
}

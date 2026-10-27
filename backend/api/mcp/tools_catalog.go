package mcp

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
)

// allTools lists every tool. Adding a feature to the app means adding its tool
// here; the contract in docs/API-CONTRACT.md lists the same set.
func (s *Server) allTools() []*tool {
	var all []*tool
	for _, group := range [][]*tool{s.accountTools(), s.diagramTools(), s.folderTools(), s.sharingTools(), s.versionTools(), s.dbmlTools()} {
		all = append(all, group...)
	}

	return all
}

func (s *Server) accountTools() []*tool {
	return []*tool{{
		Name:        "whoami",
		Description: "Return the account these tools act as (name, email) and whether the token is read-only.",
		Schema:      object(props{}),
		Run: func(c *gin.Context, who caller, _ json.RawMessage) (any, error) {
			u, err := s.app.Core.User.Get(c.Request.Context(), who.UserID)
			if err != nil {
				return nil, err
			}

			return map[string]any{"guid": u.Guid, "name": u.Name, "email": u.Email, "read_only": who.ReadOnly}, nil
		},
	}}
}

// diagramView is a diagram as shown to a model: the layout (table positions)
// is canvas geometry that only costs tokens, so it is left out.
type diagramView struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	DBML              string `json:"dbml"`
	Revision          int64  `json:"revision"`
	Role              string `json:"your_role"`
	FolderID          string `json:"folder_id"`
	OwnerName         string `json:"owner_name"`
	UpdatedByName     string `json:"updated_by_name"`
	UpdatedAt         string `json:"updated_at"`
	PublicLinkEnabled bool   `json:"public_link_enabled"`
	// PublicLinkPath is the path on the app's own host; only owners ever see it.
	PublicLinkPath string `json:"public_link_path,omitempty"`
}

func viewOf(d *model.Diagram) diagramView {
	v := diagramView{
		ID: d.Guid, Name: d.Name, DBML: d.DBML, Revision: d.Revision, Role: string(d.Role), FolderID: d.FolderID,
		OwnerName: d.OwnerName, UpdatedByName: d.UpdatedByName, UpdatedAt: d.UpdatedAt, PublicLinkEnabled: d.ShareToken != "",
	}
	if d.ShareToken != "" {
		v.PublicLinkPath = "/s/" + d.ShareToken
	}

	return v
}

type diagramItemView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	FolderID  string `json:"folder_id"`
	OwnerName string `json:"owner_name"`
	Role      string `json:"your_role"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Server) diagramTools() []*tool {
	idArg := props{"diagram_id": str("The diagram's id (a UUID from list_diagrams).")}

	return []*tool{
		{
			Name:        "list_diagrams",
			Description: "List the diagrams you can access (own, shared with you, or inside a shared folder), newest first. Does not include DBML; use get_diagram for that.",
			Schema: object(props{
				"search":    str("Only diagrams whose name contains this text."),
				"folder_id": str("Only diagrams directly in this folder. Use an empty string for diagrams not in any folder."),
				"page":      integer("Page number, from 1."),
				"limit":     integer("Page size, 1 to 100 (default 20)."),
			}),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					Search   string  `json:"search"`
					FolderID *string `json:"folder_id"`
					Page     int32   `json:"page"`
					Limit    int32   `json:"limit"`
				}](raw)
				if err != nil {
					return nil, err
				}

				var filters []model.Filter
				if a.FolderID != nil {
					filters = append(filters, model.Filter{Field: "folder_id", Type: "eq", Value: *a.FolderID})
				}
				if a.Limit == 0 {
					a.Limit = 20
				}
				a.Limit = min(a.Limit, 100)

				res, err := s.app.Diagrams.Diagram.Find(c.Request.Context(), who.UserID, diagrams.FindDiagramsParams{
					Search: a.Search, Filters: filters, Page: max(a.Page, 1), Limit: a.Limit,
					Sort: []model.SortBy{{Field: "updated_at", Type: "desc"}},
				})
				if err != nil {
					return nil, err
				}

				items := make([]diagramItemView, 0, len(res.Items))
				for _, i := range res.Items {
					items = append(items, diagramItemView{ID: i.Guid, Name: i.Name, FolderID: i.FolderID, OwnerName: i.OwnerName, Role: string(i.Role), UpdatedAt: i.UpdatedAt})
				}

				return map[string]any{"total": res.Count, "diagrams": items}, nil
			},
		},
		{
			Name:        "get_diagram",
			Description: "Get one diagram: its name, full DBML source, revision and your role. Always read it before editing and keep the revision for update_diagram.",
			Schema:      object(idArg, "diagram_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID string `json:"diagram_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				d, err := s.app.Diagrams.Diagram.Get(c.Request.Context(), who.UserID, a.DiagramID)
				if err != nil {
					return nil, err
				}

				return viewOf(d), nil
			},
		},
		{
			Name:        "create_diagram",
			Description: "Create a diagram you will own. dbml is optional; use validate_dbml first if unsure it is correct.",
			Write:       true,
			Schema: object(props{
				"name":      str("Diagram name, 1 to 120 characters."),
				"dbml":      str("DBML source (up to 1 MiB)."),
				"folder_id": str("Put it in this folder (you need editor access to it). Omit for top level."),
			}, "name"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					Name     string `json:"name"`
					DBML     string `json:"dbml"`
					FolderID string `json:"folder_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				d, err := s.app.Diagrams.Diagram.Create(c.Request.Context(), who.UserID, diagrams.CreateDiagramParams{Name: a.Name, DBML: a.DBML, FolderID: a.FolderID})
				if err != nil {
					return nil, err
				}

				return viewOf(d), nil
			},
		},
		{
			Name: "update_diagram",
			Description: "Change a diagram's name and/or replace its whole DBML. Pass base_revision (from get_diagram) whenever you send dbml: if somebody changed the diagram meanwhile the call fails instead of overwriting them. " +
				"A new history snapshot is kept automatically.",
			Write:       true,
			Destructive: true,
			Schema: object(props{
				"diagram_id":    str("The diagram's id."),
				"name":          str("New name."),
				"dbml":          str("The complete new DBML (it replaces the old one)."),
				"base_revision": integer("The revision you last read; required in practice when sending dbml."),
			}, "diagram_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID    string  `json:"diagram_id"`
					Name         *string `json:"name"`
					DBML         *string `json:"dbml"`
					BaseRevision *int64  `json:"base_revision"`
				}](raw)
				if err != nil {
					return nil, err
				}
				if a.Name == nil && a.DBML == nil {
					return nil, badInput("send name and/or dbml")
				}

				d, err := s.app.Diagrams.Diagram.Update(c.Request.Context(), who.UserID, diagrams.UpdateDiagramParams{
					Guid: a.DiagramID, Name: a.Name, DBML: a.DBML, BaseRevision: a.BaseRevision,
				})
				if err != nil {
					return nil, err
				}

				return viewOf(d), nil
			},
		},
		{
			Name:        "delete_diagram",
			Description: "Delete a diagram (owner only). Its public link stops working. This cannot be undone from the API.",
			Write:       true,
			Destructive: true,
			Schema:      object(idArg, "diagram_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID string `json:"diagram_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				if err := s.app.Diagrams.Diagram.Delete(c.Request.Context(), who.UserID, a.DiagramID); err != nil {
					return nil, err
				}

				return map[string]any{"deleted": a.DiagramID}, nil
			},
		},
		{
			Name:        "move_diagram",
			Description: "Move a diagram into a folder, or to the top level with an empty folder_id. Needs owner on the diagram and editor on the destination.",
			Write:       true,
			Schema:      object(props{"diagram_id": str("The diagram's id."), "folder_id": str("Destination folder id, or empty for top level.")}, "diagram_id", "folder_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID string `json:"diagram_id"`
					FolderID  string `json:"folder_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				d, err := s.app.Diagrams.Diagram.SetFolder(c.Request.Context(), who.UserID, a.DiagramID, a.FolderID)
				if err != nil {
					return nil, err
				}

				return viewOf(d), nil
			},
		},
		{
			Name:        "set_public_link",
			Description: "Turn a diagram's public read-only link on or off (owner only). Anyone with the link can read the diagram without an account. Turning it on again keeps the same link.",
			Write:       true,
			Schema:      object(props{"diagram_id": str("The diagram's id."), "enabled": boolean("true to enable the public link, false to revoke it.")}, "diagram_id", "enabled"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID string `json:"diagram_id"`
					Enabled   *bool  `json:"enabled"`
				}](raw)
				if err != nil {
					return nil, err
				}
				if a.Enabled == nil {
					return nil, badInput("enabled is required")
				}

				d, err := s.app.Diagrams.Diagram.SetShare(c.Request.Context(), who.UserID, a.DiagramID, *a.Enabled)
				if err != nil {
					return nil, err
				}

				v := viewOf(d)

				return map[string]any{"public_link_enabled": v.PublicLinkEnabled, "public_link_path": v.PublicLinkPath}, nil
			},
		},
	}
}

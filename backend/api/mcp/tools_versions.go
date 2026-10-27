package mcp

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
)

func (s *Server) versionTools() []*tool {
	did := props{"diagram_id": str("The diagram's id.")}
	both := props{"diagram_id": str("The diagram's id."), "version_id": str("The version's id (from list_versions).")}

	return []*tool{
		{
			Name:        "list_versions",
			Description: "List a diagram's history snapshots, newest first (without their DBML). Snapshots are taken automatically when the DBML changes, and manually with create_version.",
			Schema:      object(props{"diagram_id": did["diagram_id"], "page": integer("Page number, from 1."), "limit": integer("Page size, 1 to 100 (default 20).")}, "diagram_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID string `json:"diagram_id"`
					Page      int32  `json:"page"`
					Limit     int32  `json:"limit"`
				}](raw)
				if err != nil {
					return nil, err
				}
				if a.Limit == 0 {
					a.Limit = 20
				}

				res, err := s.app.Diagrams.Version.Find(c.Request.Context(), who.UserID, diagrams.FindVersionsParams{DiagramGuid: a.DiagramID, Page: max(a.Page, 1), Limit: min(a.Limit, 100)})
				if err != nil {
					return nil, err
				}

				return map[string]any{"total": res.Count, "versions": res.Items}, nil
			},
		},
		{
			Name:        "get_version",
			Description: "Get one history snapshot including its DBML, to compare with or restore.",
			Schema:      object(both, "diagram_id", "version_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID string `json:"diagram_id"`
					VersionID string `json:"version_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				v, err := s.app.Diagrams.Version.Get(c.Request.Context(), who.UserID, a.DiagramID, a.VersionID)
				if err != nil {
					return nil, err
				}

				return map[string]any{"id": v.Guid, "label": v.Label, "created_at": v.CreatedAt, "dbml": v.DBML}, nil
			},
		},
		{
			Name:        "create_version",
			Description: "Save a named snapshot of the diagram's current saved state (editor or owner).",
			Write:       true,
			Schema:      object(props{"diagram_id": did["diagram_id"], "label": str("Optional label, up to 120 characters.")}, "diagram_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID string `json:"diagram_id"`
					Label     string `json:"label"`
				}](raw)
				if err != nil {
					return nil, err
				}

				return s.app.Diagrams.Version.Create(c.Request.Context(), who.UserID, a.DiagramID, a.Label)
			},
		},
		{
			Name:        "restore_version",
			Description: "Replace the diagram's DBML with a snapshot's (the name is kept). The current state is saved as a new \"Before restore\" version first, so this can be undone.",
			Write:       true,
			Schema:      object(both, "diagram_id", "version_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID string `json:"diagram_id"`
					VersionID string `json:"version_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				d, err := s.app.Diagrams.Version.Restore(c.Request.Context(), who.UserID, a.DiagramID, a.VersionID)
				if err != nil {
					return nil, err
				}

				return viewOf(d), nil
			},
		},
		{
			Name:        "delete_version",
			Description: "Delete one history snapshot (editor or owner).",
			Write:       true,
			Destructive: true,
			Schema:      object(both, "diagram_id", "version_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					DiagramID string `json:"diagram_id"`
					VersionID string `json:"version_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				if err := s.app.Diagrams.Version.Delete(c.Request.Context(), who.UserID, a.DiagramID, a.VersionID); err != nil {
					return nil, err
				}

				return map[string]any{"deleted": a.VersionID}, nil
			},
		},
	}
}

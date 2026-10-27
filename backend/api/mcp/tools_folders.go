package mcp

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
)

type folderView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ParentID  string `json:"parent_id"`
	OwnerName string `json:"owner_name"`
	Role      string `json:"your_role"`
}

func folderOf(f *model.Folder) folderView {
	return folderView{ID: f.Guid, Name: f.Name, ParentID: f.ParentID, OwnerName: f.OwnerName, Role: string(f.Role)}
}

func (s *Server) folderTools() []*tool {
	idArg := props{"folder_id": str("The folder's id (from list_folders).")}

	return []*tool{
		{
			Name:        "list_folders",
			Description: "List every folder you can reach as a flat list; build the tree from parent_id (empty = top level, or a parent you cannot see).",
			Schema:      object(props{"search": str("Only folders whose name contains this text.")}),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					Search string `json:"search"`
				}](raw)
				if err != nil {
					return nil, err
				}

				res, err := s.app.Diagrams.Folder.Find(c.Request.Context(), who.UserID, diagrams.FindFoldersParams{Search: a.Search, Limit: 1000})
				if err != nil {
					return nil, err
				}

				items := make([]folderView, 0, len(res.Items))
				for _, f := range res.Items {
					items = append(items, folderOf(f))
				}

				return map[string]any{"total": res.Count, "folders": items}, nil
			},
		},
		{
			Name:        "create_folder",
			Description: "Create a folder you will own, optionally inside another folder you can edit (nesting is limited to 8 levels).",
			Write:       true,
			Schema:      object(props{"name": str("Folder name, 1 to 120 characters."), "parent_id": str("Parent folder id; omit for top level.")}, "name"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					Name     string `json:"name"`
					ParentID string `json:"parent_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				f, err := s.app.Diagrams.Folder.Create(c.Request.Context(), who.UserID, a.Name, a.ParentID)
				if err != nil {
					return nil, err
				}

				return folderOf(f), nil
			},
		},
		{
			Name:        "rename_folder",
			Description: "Rename a folder (editor or owner).",
			Write:       true,
			Schema:      object(props{"folder_id": str("The folder's id."), "name": str("New name.")}, "folder_id", "name"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					FolderID string `json:"folder_id"`
					Name     string `json:"name"`
				}](raw)
				if err != nil {
					return nil, err
				}

				f, err := s.app.Diagrams.Folder.Update(c.Request.Context(), who.UserID, a.FolderID, &a.Name)
				if err != nil {
					return nil, err
				}

				return folderOf(f), nil
			},
		},
		{
			Name:        "move_folder",
			Description: "Move a folder under another folder, or to the top level with an empty parent_id. Needs owner on the folder and editor on the destination; cycles and over-deep trees are refused.",
			Write:       true,
			Schema:      object(props{"folder_id": str("The folder's id."), "parent_id": str("New parent folder id, or empty for top level.")}, "folder_id", "parent_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					FolderID string `json:"folder_id"`
					ParentID string `json:"parent_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				f, err := s.app.Diagrams.Folder.SetParent(c.Request.Context(), who.UserID, a.FolderID, a.ParentID)
				if err != nil {
					return nil, err
				}

				return folderOf(f), nil
			},
		},
		{
			Name:        "delete_folder",
			Description: "Delete an EMPTY folder (owner only). A folder that still holds folders or diagrams is refused: move or delete its contents first.",
			Write:       true,
			Destructive: true,
			Schema:      object(idArg, "folder_id"),
			Run: func(c *gin.Context, who caller, raw json.RawMessage) (any, error) {
				a, err := bind[struct {
					FolderID string `json:"folder_id"`
				}](raw)
				if err != nil {
					return nil, err
				}

				if err := s.app.Diagrams.Folder.Delete(c.Request.Context(), who.UserID, a.FolderID); err != nil {
					return nil, err
				}

				return map[string]any{"deleted": a.FolderID}, nil
			},
		},
	}
}

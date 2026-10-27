package dto

import "gitlab.com/udevs/dbdiagram/backend/internal/model"

type CreateFolderRequest struct {
	Name string `json:"name" binding:"required"`
	// ParentID nests the folder inside one the caller may edit; "" = top level.
	ParentID string `json:"parent_id"`
}

// ReplaceFolderRequest is PUT.
type ReplaceFolderRequest struct {
	Name string `json:"name" binding:"required"`
}

type PatchFolderRequest struct {
	Name *string `json:"name"`
}

// FolderList documents model.List[*model.Folder] for swagger; see DiagramList.
type FolderList struct {
	Count int32          `json:"count"`
	Items []model.Folder `json:"items"`
}

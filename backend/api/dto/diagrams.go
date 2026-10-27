package dto

import "gitlab.com/udevs/dbdiagram/backend/internal/model"

// CreateDiagramRequest creates a diagram; dbml and positions may be omitted for
// an empty one.
type CreateDiagramRequest struct {
	Name      string                    `json:"name" binding:"required"`
	DBML      string                    `json:"dbml"`
	Positions map[string]model.Position `json:"positions"`
	// FolderID files the new diagram in a folder the caller may edit; "" = top level.
	FolderID string `json:"folder_id"`
}

// ReplaceDiagramRequest is PUT: the whole mutable representation. BaseRevision,
// when sent with a dbml, makes the save fail with REVISION_CONFLICT if somebody
// else changed the DBML since the caller loaded it.
type ReplaceDiagramRequest struct {
	Name         string                    `json:"name" binding:"required"`
	DBML         string                    `json:"dbml"`
	Positions    map[string]model.Position `json:"positions"`
	BaseRevision *int64                    `json:"base_revision"`
}

// PatchDiagramRequest uses pointers: a field the client did not send keeps its
// stored value, and an explicit "" or {} is a real value.
type PatchDiagramRequest struct {
	Name      *string                    `json:"name"`
	DBML      *string                    `json:"dbml"`
	Positions *map[string]model.Position `json:"positions"`
	// BaseRevision: see ReplaceDiagramRequest.
	BaseRevision *int64 `json:"base_revision"`
}

type SetDiagramFolderRequest struct {
	// FolderID is the folder guid; "" takes the diagram out of its folder.
	FolderID string `json:"folder_id"`
}

type SetDiagramShareRequest struct {
	Enabled *bool `json:"enabled" binding:"required"`
}

// DiagramList documents model.List[*model.DiagramItem] for swagger, which
// cannot resolve a generic type from another package. The JSON is identical.
type DiagramList struct {
	Count int32               `json:"count"`
	Items []model.DiagramItem `json:"items"`
}

// CreateVersionRequest is optional as a whole: a bare POST takes an unlabelled snapshot.
type CreateVersionRequest struct {
	Label string `json:"label"`
}

// VersionList documents model.List[*model.VersionItem] for swagger; see DiagramList.
type VersionList struct {
	Count int32               `json:"count"`
	Items []model.VersionItem `json:"items"`
}

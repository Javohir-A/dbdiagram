// Package repo declares the repository interfaces the service layer depends on.
package repo

import (
	"context"
	"time"

	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
)

// DiagramI is the `diagrams` repository.
type DiagramI interface {
	Create(ctx context.Context, p *models.DiagramCreate) (*model.Diagram, error)
	Get(ctx context.Context, guid string) (*model.Diagram, error)
	// GetShared resolves a public link; an unknown or revoked token is NotFound.
	GetShared(ctx context.Context, token string) (*model.SharedDiagram, error)
	Find(ctx context.Context, p *models.DiagramFind) (*model.List[*model.DiagramItem], error)
	Update(ctx context.Context, patch *models.DiagramPatch) (*model.Diagram, error)
	// SetFolder files the diagram in a folder; an empty folderGuid unfiles it.
	SetFolder(ctx context.Context, guid, folderGuid string) (*model.Diagram, error)
	// SetShareToken stores the public token; an empty token revokes the link.
	SetShareToken(ctx context.Context, guid, token string) (*model.Diagram, error)
	// Revision is the cheap change marker the editor polls.
	Revision(ctx context.Context, guid string) (*model.Revision, error)
	Delete(ctx context.Context, guid string) error
	// ClaimUnowned gives every diagram created before accounts existed to owner.
	ClaimUnowned(ctx context.Context, ownerGuid string) (int64, error)
}

// FolderI is the `folders` repository.
type FolderI interface {
	Create(ctx context.Context, p *models.FolderCreate) (*model.Folder, error)
	Get(ctx context.Context, guid string) (*model.Folder, error)
	Find(ctx context.Context, p *models.FolderFind) (*model.List[*model.Folder], error)
	Update(ctx context.Context, patch *models.FolderPatch) (*model.Folder, error)
	// SetParent moves a folder; an empty parentGuid makes it top level.
	SetParent(ctx context.Context, guid, parentGuid string) (*model.Folder, error)
	Delete(ctx context.Context, guid string) error
	// HasChildren only reports the fact; refusing the delete is the service's call.
	HasChildren(ctx context.Context, guid string) (bool, error)
	// Depth counts the levels from the top through this folder (top level = 1).
	Depth(ctx context.Context, guid string) (int, error)
	// Height counts the levels of the subtree under this folder, itself included.
	Height(ctx context.Context, guid string) (int, error)
	// IsInside reports whether guid is ancestorGuid or lies anywhere below it.
	IsInside(ctx context.Context, guid, ancestorGuid string) (bool, error)
	ClaimUnowned(ctx context.Context, ownerGuid string) (int64, error)
}

// VersionI is the `diagram_versions` repository. Every read is scoped to the
// diagram guid as well as the version guid, and joins the live diagram, so a
// version can neither be reached through another diagram's key nor outlive
// its diagram.
type VersionI interface {
	Create(ctx context.Context, p *models.VersionCreate) (*model.Version, error)
	Get(ctx context.Context, diagramGuid, guid string) (*model.Version, error)
	// Latest is the newest snapshot of a diagram; NotFound when there is none.
	Latest(ctx context.Context, diagramGuid string) (*model.Version, error)
	Find(ctx context.Context, p *models.VersionFind) (*model.List[*model.VersionItem], error)
	Delete(ctx context.Context, diagramGuid, guid string) error
	// Prune soft deletes everything but the newest `keep` snapshots.
	Prune(ctx context.Context, diagramGuid string, keep int) error
}

// AccessI answers "what may this user do here?" — the one place the role rules
// live in SQL. The service decides what a role is enough for.
type AccessI interface {
	// FolderRole is the highest of: owning the folder or an ancestor, and a
	// share on the folder or an ancestor. Empty when none, or when it is gone.
	FolderRole(ctx context.Context, userGuid, folderGuid string) (model.Role, error)
	// DiagramRole adds the diagram's own owner and shares to its folder chain.
	DiagramRole(ctx context.Context, userGuid, diagramGuid string) (model.Role, error)
}

// MemberI lists and changes DIRECT shares. The owner is never a row here.
type MemberI interface {
	ListFolder(ctx context.Context, folderGuid string) ([]*model.Member, error)
	ListDiagram(ctx context.Context, diagramGuid string) ([]*model.Member, error)
	SetFolder(ctx context.Context, folderGuid, userGuid string, role model.Role) (*model.Member, error)
	SetDiagram(ctx context.Context, diagramGuid, userGuid string, role model.Role) (*model.Member, error)
	// RemoveFolder / RemoveDiagram answer NotFound when there was no such share.
	RemoveFolder(ctx context.Context, folderGuid, userGuid string) error
	RemoveDiagram(ctx context.Context, diagramGuid, userGuid string) error
}

// UserI is the `users` repository.
type UserI interface {
	Create(ctx context.Context, p *models.UserCreate) (*model.User, error)
	Get(ctx context.Context, guid string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetCredentials(ctx context.Context, email string) (*models.Credentials, error)
	GetPasswordHash(ctx context.Context, guid string) (string, error)
	UpdateName(ctx context.Context, guid, name string) (*model.User, error)
	UpdatePassword(ctx context.Context, guid, passwordHash string) error
}

// SessionI is the `sessions` repository (refresh tokens, hashed).
type SessionI interface {
	Create(ctx context.Context, p *models.SessionCreate) error
	GetByHash(ctx context.Context, tokenHash string) (*models.Session, error)
	// Revoke stamps one token and reports whether THIS call did it: false means it
	// was already revoked (used before), which the auth service treats as token reuse.
	Revoke(ctx context.Context, guid string) (bool, error)
	RevokeFamily(ctx context.Context, familyGuid string) error
	// RevokeAllForUser ends every login of the user except exceptFamily ("" = none).
	RevokeAllForUser(ctx context.Context, userGuid, exceptFamily string) error
}

// TokenI is the `access_tokens` repository. "Active" always means not revoked
// and not expired at `now`, which the caller supplies so tests control time.
type TokenI interface {
	Create(ctx context.Context, p *models.TokenCreate) (*model.AccessToken, error)
	ListActive(ctx context.Context, userGuid string, now time.Time) ([]*model.AccessToken, error)
	CountActive(ctx context.Context, userGuid string, now time.Time) (int, error)
	// GetActiveByHash is the authentication lookup; NotFound for anything unusable.
	GetActiveByHash(ctx context.Context, tokenHash string, now time.Time) (*models.TokenAuth, error)
	// Touch records a use; a no-op if the last one is newer than `before`.
	Touch(ctx context.Context, guid string, now, before time.Time) error
	// Revoke is scoped to the owner: someone else's token is NotFound.
	Revoke(ctx context.Context, userGuid, guid string) error
}

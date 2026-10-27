// Package models holds the parameter structs only storage sees.
package models

import (
	"time"

	"gitlab.com/udevs/dbdiagram/backend/internal/model"
)

type DiagramCreate struct {
	Name       string
	DBML       string
	Positions  map[string]model.Position
	FolderGuid string // "" = top level
	OwnerGuid  string
}

// DiagramPatch writes only the fields that are set: a nil pointer means the
// column keeps its value.
type DiagramPatch struct {
	Guid      string
	Name      *string
	DBML      *string
	Positions *map[string]model.Position
	UpdatedBy string
	// BaseRevision, with DBML, makes the write conditional: it only happens if
	// the stored revision is still this one (optimistic concurrency).
	BaseRevision *int64
}

type DiagramFind struct {
	// ActorGuid scopes the list to what this user can reach, and fills Role.
	ActorGuid string
	Search    string
	Filters   []model.Filter
	Sort      []model.SortBy
	Page      int32
	Limit     int32
}

type FolderCreate struct {
	Name       string
	ParentGuid string // "" = top level
	OwnerGuid  string
}

type FolderPatch struct {
	Guid string
	Name *string
}

type FolderFind struct {
	ActorGuid string
	Search    string
	Sort      []model.SortBy
	Page      int32
	Limit     int32
}

// VersionCreate carries its own CreatedAt so the service decides the time (and
// tests can control it); zero means "now" in the database.
type VersionCreate struct {
	DiagramGuid string
	Label       string
	DBML        string
	Positions   map[string]model.Position
	CreatedAt   time.Time
}

type VersionFind struct {
	DiagramGuid string
	Page        int32
	Limit       int32
}

type UserCreate struct {
	Email        string
	Name         string
	PasswordHash string
}

// Credentials is the one place a password hash travels: from the users table
// to the auth service that compares it.
type Credentials struct {
	User         model.User
	PasswordHash string
}

type SessionCreate struct {
	UserGuid   string
	FamilyGuid string
	TokenHash  string
	UserAgent  string
	ExpiresAt  time.Time
}

type Session struct {
	Guid       string
	UserGuid   string
	FamilyGuid string
	ExpiresAt  time.Time
	Revoked    bool
}

type TokenCreate struct {
	UserGuid  string
	Name      string
	Prefix    string
	TokenHash string
	Scope     model.TokenScope
	ExpiresAt time.Time
}

// TokenAuth is what authenticating a request needs about a token.
type TokenAuth struct {
	Guid       string
	UserGuid   string
	Scope      model.TokenScope
	LastUsedAt time.Time // zero when never used
}

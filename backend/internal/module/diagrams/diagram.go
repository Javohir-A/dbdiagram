package diagrams

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
)

type DiagramService struct {
	deps Deps
}

type CreateDiagramParams struct {
	Name      string
	DBML      string
	Positions map[string]model.Position
	FolderID  string // "" = top level
}

// UpdateDiagramParams serves PUT and PATCH alike: a nil field keeps its stored
// value, so PUT simply sets all three.
type UpdateDiagramParams struct {
	Guid      string
	Name      *string
	DBML      *string
	Positions *map[string]model.Position
	// BaseRevision, sent with DBML, makes the save conditional on nobody else
	// having changed the DBML since the caller loaded it.
	BaseRevision *int64
}

type FindDiagramsParams struct {
	Search  string
	Filters []model.Filter
	Sort    []model.SortBy
	Page    int32
	Limit   int32
}

func (s *DiagramService) Create(ctx context.Context, actor string, p CreateDiagramParams) (*model.Diagram, error) {
	name, err := cleanName(p.Name)
	if err != nil {
		return nil, err
	}
	if err := checkDBML(p.DBML); err != nil {
		return nil, err
	}
	if err := checkPositions(p.Positions); err != nil {
		return nil, err
	}
	if p.Positions == nil {
		p.Positions = map[string]model.Position{}
	}

	var res *model.Diagram

	err = s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		if p.FolderID != "" {
			if err := requireDestination(ctx, st, actor, p.FolderID); err != nil {
				return err
			}
		}

		d, err := st.Diagram().Create(ctx, &models.DiagramCreate{
			Name: name, DBML: p.DBML, Positions: p.Positions, FolderGuid: p.FolderID, OwnerGuid: actor,
		})
		if err != nil {
			return err
		}

		res, err = presentDiagram(ctx, st, actor, d, model.RoleOwner)

		return err
	})

	return res, err
}

func (s *DiagramService) Get(ctx context.Context, actor, guid string) (*model.Diagram, error) {
	st := s.deps.Storage

	role, err := requireDiagram(ctx, st, actor, guid, model.RoleViewer)
	if err != nil {
		return nil, err
	}
	d, err := st.Diagram().Get(ctx, guid)
	if err != nil {
		return nil, err
	}

	return presentDiagram(ctx, st, actor, d, role)
}

// Revision is the cheap "did anything change?" answer the editor polls.
func (s *DiagramService) Revision(ctx context.Context, actor, guid string) (*model.Revision, error) {
	if _, err := requireDiagram(ctx, s.deps.Storage, actor, guid, model.RoleViewer); err != nil {
		return nil, err
	}

	return s.deps.Storage.Diagram().Revision(ctx, guid)
}

// GetShared serves the public link: only the name, the DBML and the layout.
// It is the one read that needs no account.
func (s *DiagramService) GetShared(ctx context.Context, token string) (*model.SharedDiagram, error) {
	if token == "" {
		return nil, apperr.NotFound("shared diagram not found")
	}

	return s.deps.Storage.Diagram().GetShared(ctx, token)
}

func (s *DiagramService) Find(ctx context.Context, actor string, p FindDiagramsParams) (*model.List[*model.DiagramItem], error) {
	return s.deps.Storage.Diagram().Find(ctx, &models.DiagramFind{
		ActorGuid: actor, Search: p.Search, Filters: p.Filters, Sort: p.Sort, Page: p.Page, Limit: p.Limit,
	})
}

func (s *DiagramService) Update(ctx context.Context, actor string, p UpdateDiagramParams) (*model.Diagram, error) {
	patch := &models.DiagramPatch{
		Guid: p.Guid, DBML: p.DBML, Positions: p.Positions, UpdatedBy: actor, BaseRevision: p.BaseRevision,
	}

	if p.Name != nil {
		name, err := cleanName(*p.Name)
		if err != nil {
			return nil, err
		}
		patch.Name = &name
	}
	if p.DBML != nil {
		if err := checkDBML(*p.DBML); err != nil {
			return nil, err
		}
	}
	if p.Positions != nil {
		if err := checkPositions(*p.Positions); err != nil {
			return nil, err
		}
	}

	var res *model.Diagram

	// One transaction: the access check, the snapshot of the state about to be
	// overwritten, and the write either all happen or none do.
	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		role, err := requireDiagram(ctx, st, actor, p.Guid, model.RoleEditor)
		if err != nil {
			return err
		}

		if p.DBML != nil {
			cur, err := st.Diagram().Get(ctx, p.Guid)
			if err != nil {
				return err
			}
			if cur.DBML != *p.DBML {
				if err := captureIfDue(ctx, st, cur, s.deps.now()); err != nil {
					return err
				}
			}
		}

		d, err := st.Diagram().Update(ctx, patch)
		if err != nil {
			return err
		}

		res, err = presentDiagram(ctx, st, actor, d, role)

		return err
	})

	return res, err
}

// SetFolder moves the diagram. Moving needs owner on the diagram and editor on
// the destination; an empty folderGuid makes it top level.
func (s *DiagramService) SetFolder(ctx context.Context, actor, guid, folderGuid string) (*model.Diagram, error) {
	var res *model.Diagram

	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		role, err := requireDiagram(ctx, st, actor, guid, model.RoleOwner)
		if err != nil {
			return err
		}
		if folderGuid != "" {
			if err := requireDestination(ctx, st, actor, folderGuid); err != nil {
				return err
			}
		}

		d, err := st.Diagram().SetFolder(ctx, guid, folderGuid)
		if err != nil {
			return err
		}

		res, err = presentDiagram(ctx, st, actor, d, role)

		return err
	})

	return res, err
}

// SetShare turns the public link on or off (owner only). Turning it on twice
// keeps the token, so a link already handed out stays valid.
func (s *DiagramService) SetShare(ctx context.Context, actor, guid string, enabled bool) (*model.Diagram, error) {
	var res *model.Diagram

	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		role, err := requireDiagram(ctx, st, actor, guid, model.RoleOwner)
		if err != nil {
			return err
		}
		d, err := st.Diagram().Get(ctx, guid)
		if err != nil {
			return err
		}

		if enabled != (d.ShareToken != "") {
			var token string
			if enabled {
				raw := make([]byte, config.ShareTokenBytes)
				if _, err := rand.Read(raw); err != nil {
					return apperr.Internal(err, "generate share token")
				}
				token = hex.EncodeToString(raw)
			}

			if d, err = st.Diagram().SetShareToken(ctx, guid, token); err != nil {
				return err
			}
		}

		res, err = presentDiagram(ctx, st, actor, d, role)

		return err
	})

	return res, err
}

func (s *DiagramService) Delete(ctx context.Context, actor, guid string) error {
	return s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		if _, err := requireDiagram(ctx, st, actor, guid, model.RoleOwner); err != nil {
			return err
		}

		return st.Diagram().Delete(ctx, guid)
	})
}

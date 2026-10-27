package diagrams

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
)

type VersionService struct {
	deps Deps
}

type FindVersionsParams struct {
	DiagramGuid string
	Page        int32
	Limit       int32
}

func (s *VersionService) Find(ctx context.Context, actor string, p FindVersionsParams) (*model.List[*model.VersionItem], error) {
	// Also answers 404 for a missing diagram instead of an empty list.
	if _, err := requireDiagram(ctx, s.deps.Storage, actor, p.DiagramGuid, model.RoleViewer); err != nil {
		return nil, err
	}

	return s.deps.Storage.Version().Find(ctx, &models.VersionFind{DiagramGuid: p.DiagramGuid, Page: p.Page, Limit: p.Limit})
}

func (s *VersionService) Get(ctx context.Context, actor, diagramGuid, guid string) (*model.Version, error) {
	if _, err := requireDiagram(ctx, s.deps.Storage, actor, diagramGuid, model.RoleViewer); err != nil {
		return nil, err
	}

	return s.deps.Storage.Version().Get(ctx, diagramGuid, guid)
}

// Create takes a manual snapshot of the saved state, whatever its age.
func (s *VersionService) Create(ctx context.Context, actor, diagramGuid, label string) (*model.VersionItem, error) {
	label = strings.TrimSpace(label)
	if utf8.RuneCountInString(label) > config.MaxLabelLength {
		return nil, apperr.InvalidArgument("label must be at most %d characters", config.MaxLabelLength)
	}
	if label == "" {
		label = config.ManualVersionLabel
	}

	var res *model.Version

	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		if _, err := requireDiagram(ctx, st, actor, diagramGuid, model.RoleEditor); err != nil {
			return err
		}
		cur, err := st.Diagram().Get(ctx, diagramGuid)
		if err != nil {
			return err
		}

		res, err = snapshot(ctx, st, cur, label, s.deps.now())

		return err
	})
	if err != nil {
		return nil, err
	}

	return &model.VersionItem{Guid: res.Guid, DiagramID: res.DiagramID, Label: res.Label, CreatedAt: res.CreatedAt}, nil
}

// Restore puts a snapshot's DBML and layout back. The current state is kept as
// a snapshot first, so a restore can itself be undone; the name is not touched.
func (s *VersionService) Restore(ctx context.Context, actor, diagramGuid, guid string) (*model.Diagram, error) {
	var res *model.Diagram

	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		role, err := requireDiagram(ctx, st, actor, diagramGuid, model.RoleEditor)
		if err != nil {
			return err
		}
		v, err := st.Version().Get(ctx, diagramGuid, guid)
		if err != nil {
			return err
		}
		cur, err := st.Diagram().Get(ctx, diagramGuid)
		if err != nil {
			return err
		}

		if cur.DBML != v.DBML {
			if err := preserve(ctx, st, cur, "Before restore", s.deps.now()); err != nil {
				return err
			}
		}

		d, err := st.Diagram().Update(ctx, &models.DiagramPatch{Guid: diagramGuid, DBML: &v.DBML, Positions: &v.Positions, UpdatedBy: actor})
		if err != nil {
			return err
		}

		res, err = presentDiagram(ctx, st, actor, d, role)

		return err
	})

	return res, err
}

func (s *VersionService) Delete(ctx context.Context, actor, diagramGuid, guid string) error {
	return s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		if _, err := requireDiagram(ctx, st, actor, diagramGuid, model.RoleEditor); err != nil {
			return err
		}

		return st.Version().Delete(ctx, diagramGuid, guid)
	})
}

// snapshot stores the diagram's current state as a version and trims the
// history to config.MaxVersions. Callers run it inside the transaction of the
// change it precedes.
func snapshot(ctx context.Context, st storage.StorageI, cur *model.Diagram, label string, at time.Time) (*model.Version, error) {
	v, err := st.Version().Create(ctx, &models.VersionCreate{
		DiagramGuid: cur.Guid, Label: label, DBML: cur.DBML, Positions: cur.Positions, CreatedAt: at,
	})
	if err != nil {
		return nil, err
	}

	return v, st.Version().Prune(ctx, cur.Guid, config.MaxVersions)
}

// preserve snapshots the current state unless the newest snapshot already holds
// exactly this DBML, or there is nothing worth keeping yet.
func preserve(ctx context.Context, st storage.StorageI, cur *model.Diagram, label string, at time.Time) error {
	if cur.DBML == "" {
		return nil
	}

	latest, err := st.Version().Latest(ctx, cur.Guid)
	if err == nil && latest.DBML == cur.DBML {
		return nil
	}
	if err != nil && !isNotFound(err) {
		return err
	}

	_, err = snapshot(ctx, st, cur, label, at)

	return err
}

// captureIfDue is the automatic rule behind every DBML-changing save: keep the
// state about to be overwritten, but at most once per config.VersionInterval.
func captureIfDue(ctx context.Context, st storage.StorageI, cur *model.Diagram, at time.Time) error {
	latest, err := st.Version().Latest(ctx, cur.Guid)
	if err == nil {
		made, perr := time.Parse(time.RFC3339, latest.CreatedAt)
		if perr == nil && at.Sub(made) < config.VersionInterval {
			return nil
		}
	} else if !isNotFound(err) {
		return err
	}

	return preserve(ctx, st, cur, "", at)
}

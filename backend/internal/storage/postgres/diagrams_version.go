package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/repo"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

const (
	versionColumns     = `v.guid, v.diagram_guid, v.label, v.dbml, v.positions, v.created_at`
	versionItemColumns = `v.guid, v.diagram_guid, v.label, v.created_at`

	// versionFrom joins the live diagram: a version of a deleted diagram is gone with it.
	versionFrom = `diagram_versions v JOIN diagrams d ON d.guid = v.diagram_guid AND d.deleted_at IS NULL`

	// versionOrder is newest first; guid breaks ties so pages never repeat a row.
	versionOrder = `v.created_at desc, v.guid desc`
)

type version struct {
	db     *db.Postgres
	runner db.Runner
	log    logger.Logger
}

// NewVersionRepo returns the `diagram_versions` repository.
func NewVersionRepo(pgDb *db.Postgres, runner db.Runner, log logger.Logger) repo.VersionI {
	return &version{db: pgDb, runner: runner, log: log}
}

func (r *version) Create(ctx context.Context, p *models.VersionCreate) (*model.Version, error) {
	var (
		guid = uuid.NewString()
	)

	positions, err := encodePositions(p.Positions)
	if err != nil {
		return nil, err
	}

	columns := []string{"guid", "diagram_guid", "label", "dbml", "positions"}
	values := []any{guid, p.DiagramGuid, p.Label, p.DBML, positions}
	if !p.CreatedAt.IsZero() {
		columns, values = append(columns, "created_at"), append(values, p.CreatedAt)
	}

	query := r.db.Builder.Insert("diagram_versions").Columns(columns...).Values(values...)

	if _, err := query.RunWith(r.runner).ExecContext(ctx); err != nil {
		return nil, HandleDatabaseError(err, r.log, "create version")
	}

	return r.Get(ctx, p.DiagramGuid, guid)
}

func (r *version) Get(ctx context.Context, diagramGuid, guid string) (*model.Version, error) {
	query := r.db.Builder.Select(versionColumns).From(versionFrom).
		Where(NotDeleted("v")).
		Where(squirrel.Eq{"v.guid": guid, "v.diagram_guid": diagramGuid})

	res, err := scanVersion(query.RunWith(r.runner).QueryRowContext(ctx))
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get version")
	}

	return res, nil
}

func (r *version) Latest(ctx context.Context, diagramGuid string) (*model.Version, error) {
	query := r.db.Builder.Select(versionColumns).From(versionFrom).
		Where(NotDeleted("v")).
		Where(squirrel.Eq{"v.diagram_guid": diagramGuid}).
		OrderBy(versionOrder).Limit(1)

	res, err := scanVersion(query.RunWith(r.runner).QueryRowContext(ctx))
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get latest version")
	}

	return res, nil
}

func (r *version) Find(ctx context.Context, p *models.VersionFind) (*model.List[*model.VersionItem], error) {
	var (
		res   = &model.List[*model.VersionItem]{Items: []*model.VersionItem{}}
		where = squirrel.And{NotDeleted("v"), squirrel.Eq{"v.diagram_guid": p.DiagramGuid}}
	)

	offset, limit := PrepareLimit(p.Page, p.Limit, config.DefaultLimit, config.MaxLimit)

	rows, err := r.db.Builder.Select(versionItemColumns).From(versionFrom).
		Where(where).OrderBy(versionOrder).Offset(offset).Limit(limit).
		RunWith(r.runner).QueryContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "find versions")
	}
	defer rows.Close()

	for rows.Next() {
		var (
			item      = &model.VersionItem{}
			createdAt sql.NullTime
		)
		if err := rows.Scan(&item.Guid, &item.DiagramID, &item.Label, &createdAt); err != nil {
			return nil, HandleDatabaseError(err, r.log, "scan version")
		}
		item.CreatedAt = TimeString(createdAt)
		res.Items = append(res.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, HandleDatabaseError(err, r.log, "rows error version")
	}

	count := r.db.Builder.Select("count(1)").From(versionFrom).Where(where)
	if err := count.RunWith(r.runner).QueryRowContext(ctx).Scan(&res.Count); err != nil {
		return nil, HandleDatabaseError(err, r.log, "count versions")
	}

	return res, nil
}

func (r *version) Delete(ctx context.Context, diagramGuid, guid string) error {
	query := r.db.Builder.Update("diagram_versions").
		Set("deleted_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"guid": guid, "diagram_guid": diagramGuid, "deleted_at": nil})

	result, err := query.RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return HandleDatabaseError(err, r.log, "delete version")
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return apperr.NotFound("version not found")
	}

	return nil
}

func (r *version) Prune(ctx context.Context, diagramGuid string, keep int) error {
	query := r.db.Builder.Update("diagram_versions").
		Set("deleted_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"diagram_guid": diagramGuid, "deleted_at": nil}).
		Where(squirrel.Expr(`guid IN (
			SELECT guid FROM diagram_versions
			WHERE diagram_guid = ? AND deleted_at IS NULL
			ORDER BY created_at DESC, guid DESC OFFSET ?)`, diagramGuid, keep))

	if _, err := query.RunWith(r.runner).ExecContext(ctx); err != nil {
		return HandleDatabaseError(err, r.log, "prune versions")
	}

	return nil
}

func scanVersion(row rowScanner) (*model.Version, error) {
	var (
		res       = &model.Version{}
		positions []byte
		createdAt sql.NullTime
	)

	if err := row.Scan(&res.Guid, &res.DiagramID, &res.Label, &res.DBML, &positions, &createdAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(positions, &res.Positions); err != nil {
		return nil, apperr.Internal(err, "decode positions")
	}
	if res.Positions == nil {
		res.Positions = map[string]model.Position{}
	}

	res.CreatedAt = TimeString(createdAt)

	return res, nil
}

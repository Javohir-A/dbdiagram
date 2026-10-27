package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
	// diagramColumns is one full row. Role is not here: it depends on who asks,
	// so the service fills it after checking access.
	diagramColumns = `d.guid, d.name, d.dbml, d.positions, d.folder_guid, d.share_token,
		d.owner_guid, COALESCE(o.name, ''), d.revision, COALESCE(ub.name, ''), d.created_at, d.updated_at`
	diagramFrom = `diagrams d LEFT JOIN users o ON o.guid = d.owner_guid LEFT JOIN users ub ON ub.guid = d.updated_by_guid`

	// A list row, for an actor: the role, and a folder / token only when the
	// actor may see them. `rank` comes from diagramRank.
	diagramItemFrom = `diagrams d
		LEFT JOIN facc fa ON fa.guid = d.folder_guid
		LEFT JOIN users o ON o.guid = d.owner_guid`
)

var diagramItemCols = fmt.Sprintf(`d.guid, d.name,
	CASE WHEN fa.guid IS NULL THEN NULL ELSE d.folder_guid END,
	CASE WHEN %[1]s = 3 THEN d.share_token END,
	d.owner_guid, COALESCE(o.name, ''), `+rankSQL+`, d.created_at, d.updated_at`, diagramRank, diagramRank)

// diagramFilterable and diagramSortable are the public field name -> SQL
// expression registries. Nothing the client sends reaches SQL outside them.
var (
	diagramFilterable = map[string]Field{
		"name":       {Expr: "d.name", Kind: KindText},
		"folder_id":  {Expr: "d.folder_guid", Kind: KindGuid},
		"owner_id":   {Expr: "d.owner_guid", Kind: KindGuid},
		"created_at": {Expr: "d.created_at", Kind: KindTime},
		"updated_at": {Expr: "d.updated_at", Kind: KindTime},
	}
	diagramSortable = map[string]string{
		"name":       "lower(d.name)",
		"created_at": "d.created_at",
		"updated_at": "d.updated_at",
	}
)

type diagram struct {
	db     *db.Postgres
	runner db.Runner
	log    logger.Logger
}

// NewDiagramRepo returns the `diagrams` repository.
func NewDiagramRepo(pgDb *db.Postgres, runner db.Runner, log logger.Logger) repo.DiagramI {
	return &diagram{db: pgDb, runner: runner, log: log}
}

func (r *diagram) Create(ctx context.Context, p *models.DiagramCreate) (*model.Diagram, error) {
	var (
		guid = uuid.NewString()
	)

	positions, err := encodePositions(p.Positions)
	if err != nil {
		return nil, err
	}

	query := r.db.Builder.Insert("diagrams").
		Columns("guid", "name", "dbml", "positions", "folder_guid", "owner_guid", "updated_by_guid").
		Values(guid, p.Name, p.DBML, positions, db.NullString(p.FolderGuid), db.NullString(p.OwnerGuid), db.NullString(p.OwnerGuid))

	if _, err := query.RunWith(r.runner).ExecContext(ctx); err != nil {
		return nil, HandleDatabaseError(err, r.log, "create diagram")
	}

	return r.Get(ctx, guid)
}

func (r *diagram) Get(ctx context.Context, guid string) (*model.Diagram, error) {
	query := r.db.Builder.Select(diagramColumns).From(diagramFrom).
		Where(NotDeleted("d")).
		Where(squirrel.Eq{"d.guid": guid})

	res, err := scanDiagram(query.RunWith(r.runner).QueryRowContext(ctx))
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get diagram")
	}

	return res, nil
}

func (r *diagram) Revision(ctx context.Context, guid string) (*model.Revision, error) {
	var (
		res       = &model.Revision{}
		updatedAt sql.NullTime
	)

	query := r.db.Builder.Select("d.revision", "d.updated_at", "COALESCE(ub.name, '')").
		From("diagrams d LEFT JOIN users ub ON ub.guid = d.updated_by_guid").
		Where(NotDeleted("d")).Where(squirrel.Eq{"d.guid": guid})
	if err := query.RunWith(r.runner).QueryRowContext(ctx).Scan(&res.Revision, &updatedAt, &res.UpdatedByName); err != nil {
		return nil, HandleDatabaseError(err, r.log, "get revision")
	}

	res.UpdatedAt = TimeString(updatedAt)

	return res, nil
}

func (r *diagram) GetShared(ctx context.Context, token string) (*model.SharedDiagram, error) {
	var (
		res       = &model.SharedDiagram{}
		positions []byte
	)

	query := r.db.Builder.Select("d.name", "d.dbml", "d.positions").From("diagrams d").
		Where(NotDeleted("d")).
		Where(squirrel.Eq{"d.share_token": token})

	if err := query.RunWith(r.runner).QueryRowContext(ctx).Scan(&res.Name, &res.DBML, &positions); err != nil {
		return nil, HandleDatabaseError(err, r.log, "get shared diagram")
	}
	if err := json.Unmarshal(positions, &res.Positions); err != nil {
		return nil, apperr.Internal(err, "decode positions")
	}

	return res, nil
}

// Find lists what the actor can reach: diagrams they own, were shared directly,
// or that sit in a folder they own or were shared (anywhere up its chain).
func (r *diagram) Find(ctx context.Context, p *models.DiagramFind) (*model.List[*model.DiagramItem], error) {
	var (
		res = &model.List[*model.DiagramItem]{Items: []*model.DiagramItem{}}
	)

	where := squirrel.And{NotDeleted("d"), squirrel.Expr(diagramRank + " > 0")}
	if p.Search != "" {
		where = append(where, squirrel.Expr(`d.name ILIKE ? ESCAPE '\'`, Contains(p.Search)))
	}

	filters, err := ApplyFilters(p.Filters, diagramFilterable)
	if err != nil {
		return nil, err
	}
	where = append(where, filters...)

	order, err := ApplySort(p.Sort, diagramSortable, "d.updated_at desc", "d.guid asc")
	if err != nil {
		return nil, err
	}

	offset, limit := PrepareLimit(p.Page, p.Limit, config.DefaultLimit, config.MaxLimit)

	rows, err := r.db.Builder.Select(diagramItemCols).Prefix(accessPrefix, p.ActorGuid, accessDepthCap).From(diagramItemFrom).
		Where(where).OrderBy(order).Offset(offset).Limit(limit).
		RunWith(r.runner).QueryContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "find diagrams")
	}
	defer rows.Close()

	for rows.Next() {
		item, err := scanDiagramItem(rows)
		if err != nil {
			return nil, HandleDatabaseError(err, r.log, "scan diagram")
		}
		res.Items = append(res.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, HandleDatabaseError(err, r.log, "rows error diagram")
	}

	count := r.db.Builder.Select("count(1)").Prefix(accessPrefix, p.ActorGuid, accessDepthCap).From(diagramItemFrom).Where(where)
	if err := count.RunWith(r.runner).QueryRowContext(ctx).Scan(&res.Count); err != nil {
		return nil, HandleDatabaseError(err, r.log, "count diagrams")
	}

	return res, nil
}

// Update writes only the fields the caller sent. A DBML change bumps the
// revision; with BaseRevision the write is conditional on it (the check and the
// write are one statement, so two editors cannot both win).
func (r *diagram) Update(ctx context.Context, patch *models.DiagramPatch) (*model.Diagram, error) {
	var (
		mp = map[string]any{}
	)

	if patch.Name != nil {
		mp["name"] = *patch.Name
	}
	if patch.DBML != nil {
		mp["dbml"] = *patch.DBML
		mp["revision"] = squirrel.Expr("CASE WHEN dbml IS DISTINCT FROM ? THEN revision + 1 ELSE revision END", *patch.DBML)
	}
	if patch.Positions != nil {
		positions, err := encodePositions(*patch.Positions)
		if err != nil {
			return nil, err
		}
		mp["positions"] = positions
	}
	if len(mp) == 0 { // nothing to write
		return r.Get(ctx, patch.Guid)
	}
	mp["updated_by_guid"] = db.NullString(patch.UpdatedBy)

	where := squirrel.And{squirrel.Eq{"guid": patch.Guid, "deleted_at": nil}}
	if patch.DBML != nil && patch.BaseRevision != nil {
		where = append(where, squirrel.Eq{"revision": *patch.BaseRevision})
	}

	result, err := r.db.Builder.Update("diagrams").SetMap(mp).Where(where).RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "update diagram")
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		// Missing, or the revision moved on: tell them apart.
		if _, err := r.Get(ctx, patch.Guid); err != nil {
			return nil, err
		}

		return nil, apperr.RevisionConflict("this diagram was changed by someone else since you loaded it")
	}

	return r.Get(ctx, patch.Guid)
}

func (r *diagram) SetFolder(ctx context.Context, guid, folderGuid string) (*model.Diagram, error) {
	return r.write(ctx, guid, map[string]any{"folder_guid": db.NullString(folderGuid)}, "set diagram folder")
}

func (r *diagram) SetShareToken(ctx context.Context, guid, token string) (*model.Diagram, error) {
	return r.write(ctx, guid, map[string]any{"share_token": db.NullString(token)}, "set diagram share token")
}

// write applies a column map to one live row and returns it. updated_at is
// stamped by the trigger.
func (r *diagram) write(ctx context.Context, guid string, mp map[string]any, message string) (*model.Diagram, error) {
	query := r.db.Builder.Update("diagrams").SetMap(mp).
		Where(squirrel.Eq{"guid": guid, "deleted_at": nil})

	result, err := query.RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, message)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return nil, apperr.NotFound("diagram not found")
	}

	return r.Get(ctx, guid)
}

// Delete is a soft delete. The share token is dropped with it so the public
// link dies at once and the token can never come back to life.
func (r *diagram) Delete(ctx context.Context, guid string) error {
	query := r.db.Builder.Update("diagrams").
		Set("deleted_at", squirrel.Expr("now()")).
		Set("share_token", nil).
		Where(squirrel.Eq{"guid": guid, "deleted_at": nil})

	result, err := query.RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return HandleDatabaseError(err, r.log, "delete diagram")
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return apperr.NotFound("diagram not found")
	}

	return nil
}

func (r *diagram) ClaimUnowned(ctx context.Context, ownerGuid string) (int64, error) {
	result, err := r.db.Builder.Update("diagrams").Set("owner_guid", ownerGuid).
		Where(squirrel.Eq{"owner_guid": nil, "deleted_at": nil}).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return 0, HandleDatabaseError(err, r.log, "claim diagrams")
	}

	return result.RowsAffected()
}

func scanDiagram(row rowScanner) (*model.Diagram, error) {
	var (
		res                  = &model.Diagram{}
		positions            []byte
		folder, token, owner sql.NullString
		createdAt, updatedAt sql.NullTime
	)

	if err := row.Scan(&res.Guid, &res.Name, &res.DBML, &positions, &folder, &token,
		&owner, &res.OwnerName, &res.Revision, &res.UpdatedByName, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(positions, &res.Positions); err != nil {
		return nil, apperr.Internal(err, "decode positions")
	}
	if res.Positions == nil {
		res.Positions = map[string]model.Position{}
	}

	res.FolderID, res.ShareToken, res.OwnerID = folder.String, token.String, owner.String
	res.CreatedAt, res.UpdatedAt = TimeString(createdAt), TimeString(updatedAt)

	return res, nil
}

func scanDiagramItem(row rowScanner) (*model.DiagramItem, error) {
	var (
		res                  = &model.DiagramItem{}
		folder, token, owner sql.NullString
		role                 string
		createdAt, updatedAt sql.NullTime
	)

	if err := row.Scan(&res.Guid, &res.Name, &folder, &token, &owner, &res.OwnerName, &role, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	res.FolderID, res.ShareToken, res.OwnerID = folder.String, token.String, owner.String
	res.Role = model.Role(role)
	res.CreatedAt, res.UpdatedAt = TimeString(createdAt), TimeString(updatedAt)

	return res, nil
}

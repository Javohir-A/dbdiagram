package postgres

import (
	"context"
	"database/sql"

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
	// folderColumns is one full row; Role is filled by the service (it depends on who asks).
	folderColumns = `f.guid, f.name, f.parent_guid, f.owner_guid, COALESCE(o.name, ''), f.created_at, f.updated_at`
	folderFrom    = `folders f LEFT JOIN users o ON o.guid = f.owner_guid`

	// A list row: the inner join on facc IS the access filter, and the parent
	// is shown only when the actor can see it too.
	folderItemFrom = `folders f
		JOIN facc fa ON fa.guid = f.guid
		LEFT JOIN facc pa ON pa.guid = f.parent_guid
		LEFT JOIN users o ON o.guid = f.owner_guid`
)

var folderItemCols = `f.guid, f.name, CASE WHEN pa.guid IS NULL THEN NULL ELSE f.parent_guid END,
	f.owner_guid, COALESCE(o.name, ''), ` + `CASE fa.rank WHEN 3 THEN 'owner' WHEN 2 THEN 'editor' ELSE 'viewer' END, f.created_at, f.updated_at`

var folderSortable = map[string]string{
	"name":       "lower(f.name)",
	"created_at": "f.created_at",
	"updated_at": "f.updated_at",
}

type folder struct {
	db     *db.Postgres
	runner db.Runner
	log    logger.Logger
}

// NewFolderRepo returns the `folders` repository.
func NewFolderRepo(pgDb *db.Postgres, runner db.Runner, log logger.Logger) repo.FolderI {
	return &folder{db: pgDb, runner: runner, log: log}
}

func (r *folder) Create(ctx context.Context, p *models.FolderCreate) (*model.Folder, error) {
	var (
		guid = uuid.NewString()
	)

	query := r.db.Builder.Insert("folders").
		Columns("guid", "name", "parent_guid", "owner_guid").
		Values(guid, p.Name, db.NullString(p.ParentGuid), db.NullString(p.OwnerGuid))

	if _, err := query.RunWith(r.runner).ExecContext(ctx); err != nil {
		return nil, HandleDatabaseError(err, r.log, "create folder")
	}

	return r.Get(ctx, guid)
}

func (r *folder) Get(ctx context.Context, guid string) (*model.Folder, error) {
	query := r.db.Builder.Select(folderColumns).From(folderFrom).
		Where(NotDeleted("f")).
		Where(squirrel.Eq{"f.guid": guid})

	res, err := scanFolder(query.RunWith(r.runner).QueryRowContext(ctx))
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get folder")
	}

	return res, nil
}

func (r *folder) Find(ctx context.Context, p *models.FolderFind) (*model.List[*model.Folder], error) {
	var (
		res = &model.List[*model.Folder]{Items: []*model.Folder{}}
	)

	where := squirrel.And{NotDeleted("f")}
	if p.Search != "" {
		where = append(where, squirrel.Expr(`f.name ILIKE ? ESCAPE '\'`, Contains(p.Search)))
	}

	order, err := ApplySort(p.Sort, folderSortable, "lower(f.name) asc", "f.guid asc")
	if err != nil {
		return nil, err
	}

	offset, limit := PrepareLimit(p.Page, p.Limit, config.DefaultLimit, config.MaxLimit)

	rows, err := r.db.Builder.Select(folderItemCols).Prefix(accessPrefix, p.ActorGuid, accessDepthCap).From(folderItemFrom).
		Where(where).OrderBy(order).Offset(offset).Limit(limit).
		RunWith(r.runner).QueryContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "find folders")
	}
	defer rows.Close()

	for rows.Next() {
		item, err := scanFolderItem(rows)
		if err != nil {
			return nil, HandleDatabaseError(err, r.log, "scan folder")
		}
		res.Items = append(res.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, HandleDatabaseError(err, r.log, "rows error folder")
	}

	count := r.db.Builder.Select("count(1)").Prefix(accessPrefix, p.ActorGuid, accessDepthCap).From(folderItemFrom).Where(where)
	if err := count.RunWith(r.runner).QueryRowContext(ctx).Scan(&res.Count); err != nil {
		return nil, HandleDatabaseError(err, r.log, "count folders")
	}

	return res, nil
}

func (r *folder) Update(ctx context.Context, patch *models.FolderPatch) (*model.Folder, error) {
	if patch.Name == nil { // nothing to write
		return r.Get(ctx, patch.Guid)
	}

	return r.write(ctx, patch.Guid, map[string]any{"name": *patch.Name}, "update folder")
}

func (r *folder) SetParent(ctx context.Context, guid, parentGuid string) (*model.Folder, error) {
	return r.write(ctx, guid, map[string]any{"parent_guid": db.NullString(parentGuid)}, "move folder")
}

func (r *folder) write(ctx context.Context, guid string, mp map[string]any, message string) (*model.Folder, error) {
	result, err := r.db.Builder.Update("folders").SetMap(mp).
		Where(squirrel.Eq{"guid": guid, "deleted_at": nil}).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, message)
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return nil, apperr.NotFound("folder not found")
	}

	return r.Get(ctx, guid)
}

func (r *folder) Delete(ctx context.Context, guid string) error {
	query := r.db.Builder.Update("folders").
		Set("deleted_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"guid": guid, "deleted_at": nil})

	result, err := query.RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return HandleDatabaseError(err, r.log, "delete folder")
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return apperr.NotFound("folder not found")
	}

	return nil
}

func (r *folder) HasChildren(ctx context.Context, guid string) (bool, error) {
	var has bool

	err := r.runner.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM folders WHERE parent_guid = $1 AND deleted_at IS NULL)
		OR EXISTS(SELECT 1 FROM diagrams WHERE folder_guid = $1 AND deleted_at IS NULL)`, guid).Scan(&has)
	if err != nil {
		return false, HandleDatabaseError(err, r.log, "folder has children")
	}

	return has, nil
}

// The three tree questions use the same bounded recursion as the role queries.
const folderUp = `WITH RECURSIVE up(guid, parent_guid, n) AS (
	SELECT guid, parent_guid, 1 FROM folders WHERE guid = $1 AND deleted_at IS NULL
	UNION ALL
	SELECT f.guid, f.parent_guid, up.n + 1 FROM folders f JOIN up ON f.guid = up.parent_guid
	WHERE f.deleted_at IS NULL AND up.n < 50
)`

func (r *folder) Depth(ctx context.Context, guid string) (int, error) {
	var n int
	if err := r.runner.QueryRowContext(ctx, folderUp+` SELECT COALESCE(max(n), 0) FROM up`, guid).Scan(&n); err != nil {
		return 0, HandleDatabaseError(err, r.log, "folder depth")
	}

	return n, nil
}

func (r *folder) Height(ctx context.Context, guid string) (int, error) {
	var n int

	err := r.runner.QueryRowContext(ctx, `WITH RECURSIVE down(guid, n) AS (
		SELECT guid, 1 FROM folders WHERE guid = $1 AND deleted_at IS NULL
		UNION ALL
		SELECT f.guid, d.n + 1 FROM folders f JOIN down d ON f.parent_guid = d.guid
		WHERE f.deleted_at IS NULL AND d.n < 50
	) SELECT COALESCE(max(n), 0) FROM down`, guid).Scan(&n)
	if err != nil {
		return 0, HandleDatabaseError(err, r.log, "folder height")
	}

	return n, nil
}

func (r *folder) IsInside(ctx context.Context, guid, ancestorGuid string) (bool, error) {
	var inside bool
	if err := r.runner.QueryRowContext(ctx, folderUp+` SELECT EXISTS(SELECT 1 FROM up WHERE guid = $2)`, guid, ancestorGuid).Scan(&inside); err != nil {
		return false, HandleDatabaseError(err, r.log, "folder is inside")
	}

	return inside, nil
}

func (r *folder) ClaimUnowned(ctx context.Context, ownerGuid string) (int64, error) {
	result, err := r.db.Builder.Update("folders").Set("owner_guid", ownerGuid).
		Where(squirrel.Eq{"owner_guid": nil, "deleted_at": nil}).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return 0, HandleDatabaseError(err, r.log, "claim folders")
	}

	return result.RowsAffected()
}

func scanFolder(row rowScanner) (*model.Folder, error) {
	var (
		res                  = &model.Folder{}
		parent, owner        sql.NullString
		createdAt, updatedAt sql.NullTime
	)

	if err := row.Scan(&res.Guid, &res.Name, &parent, &owner, &res.OwnerName, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	res.ParentID, res.OwnerID = parent.String, owner.String
	res.CreatedAt, res.UpdatedAt = TimeString(createdAt), TimeString(updatedAt)

	return res, nil
}

func scanFolderItem(row rowScanner) (*model.Folder, error) {
	var (
		res                  = &model.Folder{}
		parent, owner        sql.NullString
		role                 string
		createdAt, updatedAt sql.NullTime
	)

	if err := row.Scan(&res.Guid, &res.Name, &parent, &owner, &res.OwnerName, &role, &createdAt, &updatedAt); err != nil {
		return nil, err
	}

	res.ParentID, res.OwnerID, res.Role = parent.String, owner.String, model.Role(role)
	res.CreatedAt, res.UpdatedAt = TimeString(createdAt), TimeString(updatedAt)

	return res, nil
}

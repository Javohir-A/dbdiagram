package postgres

import (
	"context"
	"database/sql"

	"github.com/Masterminds/squirrel"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/repo"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

type member struct {
	db     *db.Postgres
	runner db.Runner
	log    logger.Logger
}

// NewMemberRepo returns the folder_members / diagram_members repository.
func NewMemberRepo(pgDb *db.Postgres, runner db.Runner, log logger.Logger) repo.MemberI {
	return &member{db: pgDb, runner: runner, log: log}
}

// kind names the share table and its resource column once, so the folder and
// diagram methods are one implementation.
type shareTable struct{ table, column string }

var (
	folderShares  = shareTable{"folder_members", "folder_guid"}
	diagramShares = shareTable{"diagram_members", "diagram_guid"}
)

func (r *member) list(ctx context.Context, t shareTable, resourceGuid string) ([]*model.Member, error) {
	rows, err := r.db.Builder.
		Select("m.user_guid", "u.email", "u.name", "m.role", "m.created_at").
		From(t.table + " m").
		Join("users u ON u.guid = m.user_guid AND u.deleted_at IS NULL").
		Where(squirrel.Eq{"m." + t.column: resourceGuid}).
		OrderBy("lower(u.name) asc, m.user_guid asc").
		RunWith(r.runner).QueryContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "list members")
	}
	defer rows.Close()

	out := []*model.Member{}
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, HandleDatabaseError(err, r.log, "scan member")
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, HandleDatabaseError(err, r.log, "rows error member")
	}

	return out, nil
}

func (r *member) set(ctx context.Context, t shareTable, resourceGuid, userGuid string, role model.Role) (*model.Member, error) {
	_, err := r.db.Builder.Insert(t.table).
		Columns(t.column, "user_guid", "role").
		Values(resourceGuid, userGuid, string(role)).
		Suffix("ON CONFLICT (" + t.column + ", user_guid) DO UPDATE SET role = EXCLUDED.role").
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "set member")
	}

	m, err := scanMember(r.db.Builder.
		Select("m.user_guid", "u.email", "u.name", "m.role", "m.created_at").
		From(t.table + " m").
		Join("users u ON u.guid = m.user_guid").
		Where(squirrel.Eq{"m." + t.column: resourceGuid, "m.user_guid": userGuid}).
		RunWith(r.runner).QueryRowContext(ctx))
	if err != nil {
		return nil, HandleDatabaseError(err, r.log, "get member")
	}

	return m, nil
}

func (r *member) remove(ctx context.Context, t shareTable, resourceGuid, userGuid string) error {
	result, err := r.db.Builder.Delete(t.table).
		Where(squirrel.Eq{t.column: resourceGuid, "user_guid": userGuid}).
		RunWith(r.runner).ExecContext(ctx)
	if err != nil {
		return HandleDatabaseError(err, r.log, "remove member")
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return apperr.NotFound("this person has no direct access here")
	}

	return nil
}

func (r *member) ListFolder(ctx context.Context, g string) ([]*model.Member, error) {
	return r.list(ctx, folderShares, g)
}

func (r *member) ListDiagram(ctx context.Context, g string) ([]*model.Member, error) {
	return r.list(ctx, diagramShares, g)
}

func (r *member) SetFolder(ctx context.Context, g, u string, role model.Role) (*model.Member, error) {
	return r.set(ctx, folderShares, g, u, role)
}

func (r *member) SetDiagram(ctx context.Context, g, u string, role model.Role) (*model.Member, error) {
	return r.set(ctx, diagramShares, g, u, role)
}

func (r *member) RemoveFolder(ctx context.Context, g, u string) error {
	return r.remove(ctx, folderShares, g, u)
}

func (r *member) RemoveDiagram(ctx context.Context, g, u string) error {
	return r.remove(ctx, diagramShares, g, u)
}

func scanMember(row rowScanner) (*model.Member, error) {
	var (
		res       = &model.Member{}
		role      string
		createdAt sql.NullTime
	)

	if err := row.Scan(&res.UserID, &res.Email, &res.Name, &role, &createdAt); err != nil {
		return nil, err
	}

	res.Role = model.Role(role)
	res.CreatedAt = TimeString(createdAt)

	return res, nil
}

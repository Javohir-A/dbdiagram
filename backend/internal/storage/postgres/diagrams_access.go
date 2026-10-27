package postgres

import (
	"context"

	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/repo"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

// rankSQL turns a numeric rank back into the role name the API uses.
const rankSQL = `CASE %s WHEN 3 THEN 'owner' WHEN 2 THEN 'editor' WHEN 1 THEN 'viewer' ELSE '' END`

// folderChain climbs from a folder to the top: the folder itself, its parent,
// and so on. It is the "ancestors" of every role rule. The cap only guards a
// corrupt cycle; the service keeps real trees under config.MaxFolderDepth.
const folderChain = `up(guid, parent_guid, n) AS (
	SELECT guid, parent_guid, 1 FROM folders WHERE guid = $2 AND deleted_at IS NULL
	UNION ALL
	SELECT f.guid, f.parent_guid, up.n + 1 FROM folders f JOIN up ON f.guid = up.parent_guid
	WHERE f.deleted_at IS NULL AND up.n < 50
)`

// folderRoleSQL: $1 = user, $2 = folder. Owning the folder or any ancestor is
// rank 3; a share on the folder or any ancestor is its own rank; the highest wins.
const folderRoleSQL = `WITH RECURSIVE ` + folderChain + `
SELECT COALESCE(max(r), 0) FROM (
	SELECT 3 AS r FROM folders f JOIN up ON up.guid = f.guid WHERE f.owner_guid = $1
	UNION ALL
	SELECT CASE m.role WHEN 'editor' THEN 2 ELSE 1 END FROM folder_members m JOIN up ON up.guid = m.folder_guid WHERE m.user_guid = $1
) x`

// diagramRoleSQL: $1 = user, $2 = diagram. The diagram's own owner and shares,
// plus the folder chain above it.
const diagramRoleSQL = `WITH RECURSIVE d AS (
	SELECT guid, folder_guid, owner_guid FROM diagrams WHERE guid = $2 AND deleted_at IS NULL
), up(guid, parent_guid, n) AS (
	SELECT f.guid, f.parent_guid, 1 FROM folders f JOIN d ON f.guid = d.folder_guid WHERE f.deleted_at IS NULL
	UNION ALL
	SELECT f.guid, f.parent_guid, up.n + 1 FROM folders f JOIN up ON f.guid = up.parent_guid
	WHERE f.deleted_at IS NULL AND up.n < 50
)
SELECT COALESCE(max(r), 0) FROM (
	SELECT 3 AS r FROM d WHERE d.owner_guid = $1
	UNION ALL
	SELECT CASE m.role WHEN 'editor' THEN 2 ELSE 1 END FROM diagram_members m JOIN d ON d.guid = m.diagram_guid WHERE m.user_guid = $1
	UNION ALL
	SELECT 3 FROM folders f JOIN up ON up.guid = f.guid WHERE f.owner_guid = $1
	UNION ALL
	SELECT CASE m.role WHEN 'editor' THEN 2 ELSE 1 END FROM folder_members m JOIN up ON up.guid = m.folder_guid WHERE m.user_guid = $1
) x`

// accessPrefix is the CTE the list queries start with. It binds the actor once
// (so no query repeats its argument), seeds the folders the actor owns or was
// shared, and spreads each one's rank to everything below it. `facc` is then
// "every folder the actor can reach, with their best rank". Args: actor, depth cap.
const accessPrefix = `WITH RECURSIVE
me AS (SELECT ?::uuid AS guid),
seed AS (
	SELECT f.guid, 3 AS rank FROM folders f JOIN me ON f.owner_guid = me.guid WHERE f.deleted_at IS NULL
	UNION ALL
	SELECT m.folder_guid, CASE m.role WHEN 'editor' THEN 2 ELSE 1 END
	FROM folder_members m JOIN me ON m.user_guid = me.guid JOIN folders f ON f.guid = m.folder_guid AND f.deleted_at IS NULL
),
tree(guid, rank, depth) AS (
	SELECT guid, rank, 1 FROM seed
	UNION ALL
	SELECT f.guid, t.rank, t.depth + 1 FROM folders f JOIN tree t ON f.parent_guid = t.guid
	WHERE f.deleted_at IS NULL AND t.depth < ?
),
facc AS (SELECT guid, max(rank) AS rank FROM tree GROUP BY guid)`

// accessDepthCap bounds the recursion above; real trees are far shallower.
const accessDepthCap = 50

// diagramRank is the actor's rank on one diagram row `d` joined to `facc fa`.
const diagramRank = `GREATEST(
	CASE WHEN d.owner_guid = (SELECT guid FROM me) THEN 3 ELSE 0 END,
	COALESCE((SELECT CASE dm.role WHEN 'editor' THEN 2 ELSE 1 END FROM diagram_members dm
		WHERE dm.diagram_guid = d.guid AND dm.user_guid = (SELECT guid FROM me)), 0),
	COALESCE(fa.rank, 0))`

func roleOf(rank int) model.Role {
	switch rank {
	case 3:
		return model.RoleOwner
	case 2:
		return model.RoleEditor
	case 1:
		return model.RoleViewer
	default:
		return model.RoleNone
	}
}

type access struct {
	db     *db.Postgres
	runner db.Runner
	log    logger.Logger
}

// NewAccessRepo returns the role lookups.
func NewAccessRepo(pgDb *db.Postgres, runner db.Runner, log logger.Logger) repo.AccessI {
	return &access{db: pgDb, runner: runner, log: log}
}

func (r *access) FolderRole(ctx context.Context, userGuid, folderGuid string) (model.Role, error) {
	var rank int
	if err := r.runner.QueryRowContext(ctx, folderRoleSQL, userGuid, folderGuid).Scan(&rank); err != nil {
		return model.RoleNone, HandleDatabaseError(err, r.log, "folder role")
	}

	return roleOf(rank), nil
}

func (r *access) DiagramRole(ctx context.Context, userGuid, diagramGuid string) (model.Role, error) {
	var rank int
	if err := r.runner.QueryRowContext(ctx, diagramRoleSQL, userGuid, diagramGuid).Scan(&rank); err != nil {
		return model.RoleNone, HandleDatabaseError(err, r.log, "diagram role")
	}

	return roleOf(rank), nil
}

package diagrams_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/testkit"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/pgtest"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/storagetest"
)

// The same scenario runs against the in-memory fake and against real Postgres:
// if the fake's access rules drifted from the SQL, one of the two would fail.
func TestAuthzInMemory(t *testing.T) {
	authzScenario(t, testkit.New(t, storagetest.New()))
}

func TestAuthzPostgres(t *testing.T) {
	st, _ := pgtest.Open(t, "authz")
	authzScenario(t, testkit.New(t, st))
}

func want(t *testing.T, err error, kind apperr.Kind, msg string) {
	t.Helper()

	require.Error(t, err, msg)
	_, e := apperr.HTTPStatus(err)
	require.Equal(t, kind, e.Kind, "%s: got %v", msg, err)
}

func authzScenario(t *testing.T, w *testkit.World) {
	ctx := context.Background()
	m := w.Diagrams

	var (
		olga = w.Person("olga") // owns everything below
		ed   = w.Person("ed")   // editor on F1
		vic  = w.Person("vic")  // viewer on F1, and viewer on E directly
		sam  = w.Person("sam")  // stranger
	)

	// F1 > F2 > D (olga's), and E at the top level.
	f1, err := m.Folder.Create(ctx, olga, "F1", "")
	require.NoError(t, err)
	f2, err := m.Folder.Create(ctx, olga, "F2", f1.Guid)
	require.NoError(t, err)
	d, err := m.Diagram.Create(ctx, olga, diagrams.CreateDiagramParams{Name: "D", DBML: "Table d {id int}", FolderID: f2.Guid})
	require.NoError(t, err)
	e, err := m.Diagram.Create(ctx, olga, diagrams.CreateDiagramParams{Name: "E", DBML: "Table e {id int}"})
	require.NoError(t, err)

	_, err = m.Member.SetFolder(ctx, olga, f1.Guid, "ed@example.test", model.RoleEditor)
	require.NoError(t, err)
	_, err = m.Member.SetFolder(ctx, olga, f1.Guid, "vic@example.test", model.RoleViewer)
	require.NoError(t, err)
	_, err = m.Member.SetDiagram(ctx, olga, e.Guid, "vic@example.test", model.RoleViewer)
	require.NoError(t, err)

	t.Run("a share on a folder reaches everything below it", func(t *testing.T) {
		for who, role := range map[string]model.Role{olga: model.RoleOwner, ed: model.RoleEditor, vic: model.RoleViewer} {
			got, err := m.Diagram.Get(ctx, who, d.Guid)
			require.NoError(t, err)
			require.Equal(t, role, got.Role)

			f, err := m.Folder.Get(ctx, who, f2.Guid)
			require.NoError(t, err, "the subfolder inherits the share too")
			require.Equal(t, role, f.Role)
		}
	})

	t.Run("a stranger learns nothing exists", func(t *testing.T) {
		_, err := m.Diagram.Get(ctx, sam, d.Guid)
		want(t, err, apperr.KindNotFound, "get")
		_, err = m.Diagram.Revision(ctx, sam, d.Guid)
		want(t, err, apperr.KindNotFound, "revision")
		_, err = m.Version.Find(ctx, sam, diagrams.FindVersionsParams{DiagramGuid: d.Guid})
		want(t, err, apperr.KindNotFound, "versions")
		_, err = m.Member.ListDiagram(ctx, sam, d.Guid)
		want(t, err, apperr.KindNotFound, "members")
		want(t, m.Diagram.Delete(ctx, sam, d.Guid), apperr.KindNotFound, "delete")
		_, err = m.Diagram.SetShare(ctx, sam, d.Guid, true)
		want(t, err, apperr.KindNotFound, "share")
		_, err = m.Folder.Get(ctx, sam, f1.Guid)
		want(t, err, apperr.KindNotFound, "folder get")

		name := "hacked"
		_, err = m.Diagram.Update(ctx, sam, diagrams.UpdateDiagramParams{Guid: d.Guid, Name: &name})
		want(t, err, apperr.KindNotFound, "update")

		list, err := m.Diagram.Find(ctx, sam, diagrams.FindDiagramsParams{})
		require.NoError(t, err)
		require.Zero(t, list.Count)
		folders, err := m.Folder.Find(ctx, sam, diagrams.FindFoldersParams{})
		require.NoError(t, err)
		require.Zero(t, folders.Count)
	})

	t.Run("a viewer reads but never writes", func(t *testing.T) {
		name := "x"
		_, err := m.Diagram.Update(ctx, vic, diagrams.UpdateDiagramParams{Guid: d.Guid, Name: &name})
		want(t, err, apperr.KindPermissionDenied, "update")
		_, err = m.Version.Create(ctx, vic, d.Guid, "")
		want(t, err, apperr.KindPermissionDenied, "snapshot")
		_, err = m.Diagram.Create(ctx, vic, diagrams.CreateDiagramParams{Name: "n", FolderID: f1.Guid})
		want(t, err, apperr.KindPermissionDenied, "create inside a folder they only view")
		_, err = m.Folder.Create(ctx, vic, "sub", f1.Guid)
		want(t, err, apperr.KindPermissionDenied, "create a subfolder")

		_, err = m.Version.Find(ctx, vic, diagrams.FindVersionsParams{DiagramGuid: d.Guid})
		require.NoError(t, err, "reading history is allowed")
		_, err = m.Diagram.Revision(ctx, vic, d.Guid)
		require.NoError(t, err)
	})

	t.Run("an editor edits but owner-only actions are refused", func(t *testing.T) {
		name := "renamed by ed"
		got, err := m.Diagram.Update(ctx, ed, diagrams.UpdateDiagramParams{Guid: d.Guid, Name: &name})
		require.NoError(t, err)
		require.Equal(t, "ed", got.UpdatedByName)

		_, err = m.Version.Create(ctx, ed, d.Guid, "ed's snapshot")
		require.NoError(t, err)

		want(t, m.Diagram.Delete(ctx, ed, d.Guid), apperr.KindPermissionDenied, "delete")
		_, err = m.Diagram.SetShare(ctx, ed, d.Guid, true)
		want(t, err, apperr.KindPermissionDenied, "public link")
		_, err = m.Diagram.SetFolder(ctx, ed, d.Guid, "")
		want(t, err, apperr.KindPermissionDenied, "move")
		_, err = m.Member.ListDiagram(ctx, ed, d.Guid)
		want(t, err, apperr.KindPermissionDenied, "list members")
		_, err = m.Member.SetDiagram(ctx, ed, d.Guid, "sam@example.test", model.RoleViewer)
		want(t, err, apperr.KindPermissionDenied, "share")
		want(t, m.Folder.Delete(ctx, ed, f2.Guid), apperr.KindPermissionDenied, "delete a folder")
		_, err = m.Folder.SetParent(ctx, ed, f2.Guid, "")
		want(t, err, apperr.KindPermissionDenied, "move a folder")
	})

	t.Run("what an editor creates is theirs, and the folder owner keeps full rights", func(t *testing.T) {
		made, err := m.Diagram.Create(ctx, ed, diagrams.CreateDiagramParams{Name: "by ed", DBML: "x", FolderID: f2.Guid})
		require.NoError(t, err)
		require.Equal(t, model.RoleOwner, made.Role)
		require.Equal(t, ed, made.OwnerID)

		got, err := m.Diagram.Get(ctx, olga, made.Guid)
		require.NoError(t, err)
		require.Equal(t, model.RoleOwner, got.Role, "owning an ancestor folder is owning what is inside")
		got, err = m.Diagram.Get(ctx, vic, made.Guid)
		require.NoError(t, err)
		require.Equal(t, model.RoleViewer, got.Role)

		require.NoError(t, m.Diagram.Delete(ctx, olga, made.Guid), "the folder owner can clean up")
	})

	t.Run("lists show exactly what the caller can reach", func(t *testing.T) {
		roles := func(who string) map[string]model.Role {
			list, err := m.Diagram.Find(ctx, who, diagrams.FindDiagramsParams{})
			require.NoError(t, err)
			out := map[string]model.Role{}
			for _, i := range list.Items {
				out[i.Name] = i.Role
			}

			return out
		}

		require.Equal(t, map[string]model.Role{"renamed by ed": model.RoleViewer, "E": model.RoleViewer}, roles(vic), "D by inheritance, E by its own share")
		require.Equal(t, map[string]model.Role{"renamed by ed": model.RoleEditor}, roles(ed))
		require.Equal(t, map[string]model.Role{"renamed by ed": model.RoleOwner, "E": model.RoleOwner}, roles(olga))
		require.Empty(t, roles(sam))

		folders, err := m.Folder.Find(ctx, ed, diagrams.FindFoldersParams{})
		require.NoError(t, err)
		require.Equal(t, int32(2), folders.Count, "F1 and its subfolder")
		for _, f := range folders.Items {
			require.Equal(t, model.RoleEditor, f.Role)
		}
	})

	t.Run("a folder you cannot see is not revealed through its contents", func(t *testing.T) {
		_, err := m.Member.SetDiagram(ctx, olga, d.Guid, "sam@example.test", model.RoleViewer)
		require.NoError(t, err)

		got, err := m.Diagram.Get(ctx, sam, d.Guid)
		require.NoError(t, err)
		require.Equal(t, model.RoleViewer, got.Role)
		require.Empty(t, got.FolderID, "sam cannot see F2, so the diagram looks top level")

		list, err := m.Diagram.Find(ctx, sam, diagrams.FindDiagramsParams{})
		require.NoError(t, err)
		require.Len(t, list.Items, 1)
		require.Empty(t, list.Items[0].FolderID)

		_, err = m.Folder.Get(ctx, sam, f2.Guid)
		want(t, err, apperr.KindNotFound, "the folder itself stays invisible")

		require.NoError(t, m.Member.RemoveDiagram(ctx, olga, d.Guid, sam))
		_, err = m.Diagram.Get(ctx, sam, d.Guid)
		want(t, err, apperr.KindNotFound, "access ends with the share")
	})

	t.Run("only the owner sees the public link token", func(t *testing.T) {
		on, err := m.Diagram.SetShare(ctx, olga, d.Guid, true)
		require.NoError(t, err)
		require.NotEmpty(t, on.ShareToken)

		got, err := m.Diagram.Get(ctx, ed, d.Guid)
		require.NoError(t, err)
		require.Empty(t, got.ShareToken, "an editor must not be able to read (and leak) the link")

		list, err := m.Diagram.Find(ctx, ed, diagrams.FindDiagramsParams{})
		require.NoError(t, err)
		for _, i := range list.Items {
			require.Empty(t, i.ShareToken)
		}

		shared, err := m.Diagram.GetShared(ctx, on.ShareToken)
		require.NoError(t, err, "the link itself needs no account")
		require.Equal(t, "renamed by ed", shared.Name)

		_, err = m.Diagram.SetShare(ctx, olga, d.Guid, false)
		require.NoError(t, err)
	})

	t.Run("moving needs owner on the item and editor on the destination", func(t *testing.T) {
		other, err := m.Folder.Create(ctx, vic, "vic's folder", "")
		require.NoError(t, err)

		_, err = m.Diagram.SetFolder(ctx, olga, e.Guid, other.Guid)
		want(t, err, apperr.KindInvalidArgument, "olga cannot see vic's folder")

		_, err = m.Member.SetFolder(ctx, vic, other.Guid, "olga@example.test", model.RoleViewer)
		require.NoError(t, err)
		_, err = m.Diagram.SetFolder(ctx, olga, e.Guid, other.Guid)
		want(t, err, apperr.KindPermissionDenied, "a viewer may not put things in it")

		_, err = m.Member.SetFolder(ctx, vic, other.Guid, "olga@example.test", model.RoleEditor)
		require.NoError(t, err)
		moved, err := m.Diagram.SetFolder(ctx, olga, e.Guid, other.Guid)
		require.NoError(t, err)
		require.Equal(t, other.Guid, moved.FolderID)

		back, err := m.Diagram.SetFolder(ctx, olga, e.Guid, "")
		require.NoError(t, err)
		require.Empty(t, back.FolderID)

		require.NoError(t, m.Folder.Delete(ctx, vic, other.Guid))
	})

	t.Run("folders cannot form a cycle or grow past the depth limit", func(t *testing.T) {
		_, err := m.Folder.SetParent(ctx, olga, f1.Guid, f2.Guid)
		want(t, err, apperr.KindInvalidArgument, "into its own child")
		_, err = m.Folder.SetParent(ctx, olga, f1.Guid, f1.Guid)
		want(t, err, apperr.KindInvalidArgument, "into itself")

		parent := f2.Guid // F1 > F2 is already 2 levels
		for depth := 3; depth <= config.MaxFolderDepth; depth++ {
			f, err := m.Folder.Create(ctx, olga, fmt.Sprintf("level %d", depth), parent)
			require.NoError(t, err, "level %d is allowed", depth)
			parent = f.Guid
		}
		_, err = m.Folder.Create(ctx, olga, "too deep", parent)
		want(t, err, apperr.KindInvalidArgument, "one level past the limit")

		// Moving a whole subtree counts its height, not just the folder itself.
		lone, err := m.Folder.Create(ctx, olga, "lone", "")
		require.NoError(t, err)
		_, err = m.Folder.SetParent(ctx, olga, f1.Guid, lone.Guid)
		want(t, err, apperr.KindInvalidArgument, "the F1 subtree is too tall to go one level down")
		require.Equal(t, apperr.KindNotFound, kindOf2(m.Folder.Get(ctx, sam, lone.Guid)))
		require.NoError(t, m.Folder.Delete(ctx, olga, lone.Guid))
	})

	t.Run("a folder is deleted only when empty", func(t *testing.T) {
		want(t, m.Folder.Delete(ctx, olga, f1.Guid), apperr.KindFailedPrecondition, "it holds F2")

		top, err := m.Folder.Create(ctx, olga, "scratch", "")
		require.NoError(t, err)
		require.NoError(t, m.Folder.Delete(ctx, olga, top.Guid))
	})

	t.Run("members: who can manage, who can leave", func(t *testing.T) {
		list, err := m.Member.ListFolder(ctx, olga, f1.Guid)
		require.NoError(t, err)
		require.Equal(t, model.RoleOwner, list[0].Role, "the owner is listed first")
		require.Equal(t, olga, list[0].UserID)
		require.Len(t, list, 3)

		_, err = m.Member.SetFolder(ctx, olga, f1.Guid, "nobody@example.test", model.RoleViewer)
		want(t, err, apperr.KindNotFound, "unknown email")
		_, err = m.Member.SetFolder(ctx, olga, f1.Guid, "olga@example.test", model.RoleViewer)
		want(t, err, apperr.KindInvalidArgument, "sharing with yourself")
		_, err = m.Member.SetFolder(ctx, olga, f1.Guid, "sam@example.test", model.RoleOwner)
		want(t, err, apperr.KindInvalidArgument, "ownership cannot be granted")
		_, err = m.Member.SetFolder(ctx, olga, f1.Guid, "not an email", model.RoleViewer)
		want(t, err, apperr.KindInvalidArgument, "malformed email")

		changed, err := m.Member.SetFolder(ctx, olga, f1.Guid, "VIC@example.test ", model.RoleEditor)
		require.NoError(t, err, "the email is normalised; setting again changes the role")
		require.Equal(t, model.RoleEditor, changed.Role)
		_, err = m.Member.SetFolder(ctx, olga, f1.Guid, "vic@example.test", model.RoleViewer)
		require.NoError(t, err)

		want(t, m.Member.RemoveFolder(ctx, vic, f1.Guid, ed), apperr.KindPermissionDenied, "a viewer removing someone else")
		want(t, m.Member.RemoveFolder(ctx, sam, f1.Guid, ed), apperr.KindNotFound, "a stranger removing someone")
		want(t, m.Member.RemoveFolder(ctx, sam, f1.Guid, sam), apperr.KindNotFound, "leaving something you are not in")

		require.NoError(t, m.Member.RemoveDiagram(ctx, vic, e.Guid, vic), "a member can leave")
		_, err = m.Diagram.Get(ctx, vic, e.Guid)
		want(t, err, apperr.KindNotFound, "and then no longer sees it")

		require.NoError(t, m.Member.RemoveFolder(ctx, olga, f1.Guid, ed))
		_, err = m.Diagram.Get(ctx, ed, d.Guid)
		want(t, err, apperr.KindNotFound, "revoking the folder share ends access to everything inside")
	})

	t.Run("concurrent editors: a stale save is refused", func(t *testing.T) {
		cur, err := m.Diagram.Get(ctx, olga, d.Guid)
		require.NoError(t, err)

		first, second := "Table first {id int}", "Table second {id int}"
		saved, err := m.Diagram.Update(ctx, olga, diagrams.UpdateDiagramParams{Guid: d.Guid, DBML: &first, BaseRevision: &cur.Revision})
		require.NoError(t, err)
		require.Equal(t, cur.Revision+1, saved.Revision, "a DBML change bumps the revision")

		_, err = m.Diagram.Update(ctx, olga, diagrams.UpdateDiagramParams{Guid: d.Guid, DBML: &second, BaseRevision: &cur.Revision})
		want(t, err, apperr.KindRevisionConflict, "the second writer still holds the old revision")

		still, err := m.Diagram.Get(ctx, olga, d.Guid)
		require.NoError(t, err)
		require.Equal(t, first, still.DBML, "nothing was overwritten")

		positions := map[string]model.Position{"first": {X: 1, Y: 1}}
		_, err = m.Diagram.Update(ctx, olga, diagrams.UpdateDiagramParams{Guid: d.Guid, Positions: &positions, BaseRevision: &cur.Revision})
		require.NoError(t, err, "positions are last-write-wins, never conflicts")

		same, err := m.Diagram.Update(ctx, olga, diagrams.UpdateDiagramParams{Guid: d.Guid, DBML: &first})
		require.NoError(t, err, "no base revision: the write wins")
		require.Equal(t, saved.Revision, same.Revision, "unchanged DBML does not bump the revision")

		rev, err := m.Diagram.Revision(ctx, olga, d.Guid)
		require.NoError(t, err)
		require.Equal(t, saved.Revision, rev.Revision)
	})

	t.Run("claiming gives old, ownerless rows to one account", func(t *testing.T) {
		orphan, err := w.Storage.Diagram().Create(ctx, &models.DiagramCreate{Name: "legacy", Positions: map[string]model.Position{}})
		require.NoError(t, err)

		_, err = m.Diagram.Get(ctx, olga, orphan.Guid)
		want(t, err, apperr.KindNotFound, "nobody can reach it before it is claimed")

		diagramsClaimed, _, err := m.Diagram.ClaimUnowned(ctx, "sam@example.test")
		require.NoError(t, err)
		require.Equal(t, int64(1), diagramsClaimed)

		got, err := m.Diagram.Get(ctx, sam, orphan.Guid)
		require.NoError(t, err)
		require.Equal(t, model.RoleOwner, got.Role)
	})
}

func kindOf2[T any](_ T, err error) apperr.Kind {
	_, e := apperr.HTTPStatus(err)

	return e.Kind
}

package diagrams_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/storagetest"
)

// alice is the actor of every service call here; the in-memory storage needs no
// account behind it (membership tests, which do, use testkit).
const alice = "00000000-0000-0000-0000-0000000000a1"

func newModule() *diagrams.Module {
	return diagrams.New(diagrams.Deps{Storage: storagetest.New(), Cfg: &config.Config{}})
}

func kindOf(t *testing.T, err error) apperr.Kind {
	t.Helper()

	require.Error(t, err)
	_, e := apperr.HTTPStatus(err)

	return e.Kind
}

func TestCreateDiagram(t *testing.T) {
	ctx := context.Background()
	m := newModule()

	d, err := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "  Shop  ", DBML: "Table a {id int}"})
	require.NoError(t, err)
	require.Equal(t, "Shop", d.Name, "name is trimmed")
	require.NotNil(t, d.Positions, "positions are never nil in a response")

	tooLong := strings.Repeat("я", config.MaxNameLength+1)
	cases := map[string]struct {
		p    diagrams.CreateDiagramParams
		want apperr.Kind
	}{
		"empty name":     {diagrams.CreateDiagramParams{Name: "   "}, apperr.KindInvalidArgument},
		"name too long":  {diagrams.CreateDiagramParams{Name: tooLong}, apperr.KindInvalidArgument},
		"dbml too large": {diagrams.CreateDiagramParams{Name: "x", DBML: strings.Repeat("a", config.MaxDBMLSize+1)}, apperr.KindPayloadTooLarge},
		"empty table id": {diagrams.CreateDiagramParams{Name: "x", Positions: map[string]model.Position{"": {}}}, apperr.KindInvalidArgument},
		"out of range":   {diagrams.CreateDiagramParams{Name: "x", Positions: map[string]model.Position{"a": {X: 1e9}}}, apperr.KindInvalidArgument},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := m.Diagram.Create(ctx, alice, c.p)
			require.Equal(t, c.want, kindOf(t, err))
		})
	}
}

func TestUpdateDiagramPatchKeepsOmittedFields(t *testing.T) {
	ctx := context.Background()
	m := newModule()

	d, err := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "A", DBML: "Table a {id int}", Positions: map[string]model.Position{"a": {X: 1, Y: 2}}})
	require.NoError(t, err)

	newName := "B"
	got, err := m.Diagram.Update(ctx, alice, diagrams.UpdateDiagramParams{Guid: d.Guid, Name: &newName})
	require.NoError(t, err)
	require.Equal(t, "B", got.Name)
	require.Equal(t, "Table a {id int}", got.DBML, "omitted dbml is kept")
	require.Equal(t, model.Position{X: 1, Y: 2}, got.Positions["a"], "omitted positions are kept")

	// An explicit empty value is a real value, not "omitted".
	empty, emptyPos := "", map[string]model.Position{}
	got, err = m.Diagram.Update(ctx, alice, diagrams.UpdateDiagramParams{Guid: d.Guid, DBML: &empty, Positions: &emptyPos})
	require.NoError(t, err)
	require.Empty(t, got.DBML)
	require.Empty(t, got.Positions)

	blank := " "
	_, err = m.Diagram.Update(ctx, alice, diagrams.UpdateDiagramParams{Guid: d.Guid, Name: &blank})
	require.Equal(t, apperr.KindInvalidArgument, kindOf(t, err))

	_, err = m.Diagram.Update(ctx, alice, diagrams.UpdateDiagramParams{Guid: "missing", Name: &newName})
	require.Equal(t, apperr.KindNotFound, kindOf(t, err))
}

func TestShare(t *testing.T) {
	ctx := context.Background()
	m := newModule()

	d, err := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "A", DBML: "Table a {id int}"})
	require.NoError(t, err)

	on, err := m.Diagram.SetShare(ctx, alice, d.Guid, true)
	require.NoError(t, err)
	require.Len(t, on.ShareToken, config.ShareTokenBytes*2)

	again, err := m.Diagram.SetShare(ctx, alice, d.Guid, true)
	require.NoError(t, err)
	require.Equal(t, on.ShareToken, again.ShareToken, "turning it on twice keeps the link a user already copied")

	shared, err := m.Diagram.GetShared(ctx, on.ShareToken)
	require.NoError(t, err)
	require.Equal(t, "A", shared.Name)

	off, err := m.Diagram.SetShare(ctx, alice, d.Guid, false)
	require.NoError(t, err)
	require.Empty(t, off.ShareToken)

	_, err = m.Diagram.GetShared(ctx, on.ShareToken)
	require.Equal(t, apperr.KindNotFound, kindOf(t, err), "a revoked link stops working")

	_, err = m.Diagram.GetShared(ctx, "")
	require.Equal(t, apperr.KindNotFound, kindOf(t, err), "an empty token never matches an unshared diagram")
}

func TestFolders(t *testing.T) {
	ctx := context.Background()
	m := newModule()

	d, err := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "A"})
	require.NoError(t, err)
	f, err := m.Folder.Create(ctx, alice, " Work ", "")
	require.NoError(t, err)
	require.Equal(t, "Work", f.Name)
	require.Equal(t, model.RoleOwner, f.Role)

	moved, err := m.Diagram.SetFolder(ctx, alice, d.Guid, f.Guid)
	require.NoError(t, err)
	require.Equal(t, f.Guid, moved.FolderID)

	_, err = m.Diagram.SetFolder(ctx, alice, d.Guid, "00000000-0000-0000-0000-000000000000")
	require.Equal(t, apperr.KindInvalidArgument, kindOf(t, err), "a folder that does not exist is a bad argument, not a missing diagram")

	require.Equal(t, apperr.KindFailedPrecondition, kindOf(t, m.Folder.Delete(ctx, alice, f.Guid)), "a folder that still holds something is not deleted")

	_, err = m.Diagram.SetFolder(ctx, alice, d.Guid, "")
	require.NoError(t, err)
	require.NoError(t, m.Folder.Delete(ctx, alice, f.Guid))
	require.Equal(t, apperr.KindNotFound, kindOf(t, m.Folder.Delete(ctx, alice, f.Guid)))

	name := "renamed"
	_, err = m.Folder.Update(ctx, alice, f.Guid, &name)
	require.Equal(t, apperr.KindNotFound, kindOf(t, err), "a deleted folder cannot be renamed")
}

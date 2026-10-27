package diagrams_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/module/diagrams"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/storagetest"
)

// clockModule returns a module whose clock the test moves by hand.
func clockModule() (*diagrams.Module, *time.Time) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	return diagrams.New(diagrams.Deps{Storage: storagetest.New(), Cfg: &config.Config{}, Now: func() time.Time { return now }}), &now
}

func dbmls(t *testing.T, m *diagrams.Module, guid string) []string {
	t.Helper()

	list, err := m.Version.Find(context.Background(), alice, diagrams.FindVersionsParams{DiagramGuid: guid})
	require.NoError(t, err)

	out := []string{}
	for _, item := range list.Items {
		v, err := m.Version.Get(context.Background(), alice, guid, item.Guid)
		require.NoError(t, err)
		out = append(out, v.DBML)
	}

	return out
}

func save(t *testing.T, m *diagrams.Module, guid, dbml string) {
	t.Helper()

	_, err := m.Diagram.Update(context.Background(), alice, diagrams.UpdateDiagramParams{Guid: guid, DBML: &dbml})
	require.NoError(t, err)
}

func TestAutomaticSnapshotsFollowTheInterval(t *testing.T) {
	ctx := context.Background()
	m, now := clockModule()

	d, err := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "A"})
	require.NoError(t, err)

	save(t, m, d.Guid, "v1")
	require.Empty(t, dbmls(t, m, d.Guid), "an empty document is not worth a snapshot")

	save(t, m, d.Guid, "v2")
	require.Equal(t, []string{"v1"}, dbmls(t, m, d.Guid), "the state BEFORE the save is kept")

	*now = now.Add(time.Minute)
	save(t, m, d.Guid, "v3")
	require.Equal(t, []string{"v1"}, dbmls(t, m, d.Guid), "a second save inside the interval adds nothing")

	*now = now.Add(config.VersionInterval)
	save(t, m, d.Guid, "v4")
	require.Equal(t, []string{"v3", "v1"}, dbmls(t, m, d.Guid), "after the interval the next save snapshots again, newest first")

	// None of these may create a snapshot, however old the last one is.
	*now = now.Add(time.Hour)
	save(t, m, d.Guid, "v4")
	positions := map[string]model.Position{"t": {X: 1, Y: 1}}
	name := "renamed"
	_, err = m.Diagram.Update(ctx, alice, diagrams.UpdateDiagramParams{Guid: d.Guid, Positions: &positions, Name: &name})
	require.NoError(t, err)
	require.Equal(t, []string{"v3", "v1"}, dbmls(t, m, d.Guid), "unchanged DBML, dragging tables and renaming are not history")
}

func TestManualSnapshot(t *testing.T) {
	ctx := context.Background()
	m, _ := clockModule()
	d, err := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "A", DBML: "v1"})
	require.NoError(t, err)

	item, err := m.Version.Create(ctx, alice, d.Guid, "  before launch  ")
	require.NoError(t, err)
	require.Equal(t, "before launch", item.Label)
	require.Equal(t, d.Guid, item.DiagramID)

	bare, err := m.Version.Create(ctx, alice, d.Guid, "  ")
	require.NoError(t, err, "a bare snapshot is valid, even right after another")
	require.Equal(t, config.ManualVersionLabel, bare.Label, "manual saves are never confused with automatic ones (empty label)")

	_, err = m.Version.Create(ctx, alice, d.Guid, strings.Repeat("я", config.MaxLabelLength+1))
	require.Equal(t, apperr.KindInvalidArgument, kindOf(t, err))

	_, err = m.Version.Create(ctx, alice, "missing", "x")
	require.Equal(t, apperr.KindNotFound, kindOf(t, err))
}

func TestRestoreKeepsTheCurrentStateAndTheName(t *testing.T) {
	ctx := context.Background()
	m, now := clockModule()
	d, err := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "Keep me", DBML: "v1", Positions: map[string]model.Position{"a": {X: 5, Y: 6}}})
	require.NoError(t, err)

	first, err := m.Version.Create(ctx, alice, d.Guid, "first")
	require.NoError(t, err)

	*now = now.Add(time.Hour)
	save(t, m, d.Guid, "v2")
	moved := map[string]model.Position{"a": {X: 99, Y: 99}}
	_, err = m.Diagram.Update(ctx, alice, diagrams.UpdateDiagramParams{Guid: d.Guid, Positions: &moved})
	require.NoError(t, err)

	restored, err := m.Version.Restore(ctx, alice, d.Guid, first.Guid)
	require.NoError(t, err)
	require.Equal(t, "v1", restored.DBML)
	require.Equal(t, model.Position{X: 5, Y: 6}, restored.Positions["a"], "the layout comes back too")
	require.Equal(t, "Keep me", restored.Name)

	list, err := m.Version.Find(ctx, alice, diagrams.FindVersionsParams{DiagramGuid: d.Guid})
	require.NoError(t, err)
	require.Equal(t, "Before restore", list.Items[0].Label, "the state that was replaced is kept, so the restore can be undone")
	require.Equal(t, []string{"v2", "v1"}, dbmls(t, m, d.Guid), "newest first: the replaced state, then the manual snapshot; the save of v2 added none because v1 was already held")

	// Restoring what is already current changes nothing and adds nothing.
	again, err := m.Version.Restore(ctx, alice, d.Guid, first.Guid)
	require.NoError(t, err)
	require.Equal(t, "v1", again.DBML)
	after, err := m.Version.Find(ctx, alice, diagrams.FindVersionsParams{DiagramGuid: d.Guid})
	require.NoError(t, err)
	require.Equal(t, list.Count, after.Count)
}

func TestVersionsStayInsideTheirDiagram(t *testing.T) {
	ctx := context.Background()
	m, _ := clockModule()
	a, _ := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "A", DBML: "a"})
	b, _ := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "B", DBML: "b"})
	va, err := m.Version.Create(ctx, alice, a.Guid, "")
	require.NoError(t, err)

	_, err = m.Version.Get(ctx, alice, b.Guid, va.Guid)
	require.Equal(t, apperr.KindNotFound, kindOf(t, err), "A's version is not reachable through B's key")
	_, err = m.Version.Restore(ctx, alice, b.Guid, va.Guid)
	require.Equal(t, apperr.KindNotFound, kindOf(t, err))
	require.Equal(t, apperr.KindNotFound, kindOf(t, m.Version.Delete(ctx, alice, b.Guid, va.Guid)))

	require.NoError(t, m.Version.Delete(ctx, alice, a.Guid, va.Guid))
	_, err = m.Version.Get(ctx, alice, a.Guid, va.Guid)
	require.Equal(t, apperr.KindNotFound, kindOf(t, err))

	_, err = m.Version.Find(ctx, alice, diagrams.FindVersionsParams{DiagramGuid: "missing"})
	require.Equal(t, apperr.KindNotFound, kindOf(t, err), "a list for a missing diagram is a 404, not an empty page")
}

func TestHistoryIsCapped(t *testing.T) {
	ctx := context.Background()
	m, _ := clockModule()
	d, _ := m.Diagram.Create(ctx, alice, diagrams.CreateDiagramParams{Name: "A", DBML: "v"})

	for i := 0; i < config.MaxVersions+5; i++ {
		_, err := m.Version.Create(ctx, alice, d.Guid, "")
		require.NoError(t, err)
	}

	list, err := m.Version.Find(ctx, alice, diagrams.FindVersionsParams{DiagramGuid: d.Guid})
	require.NoError(t, err)
	require.Equal(t, int32(config.MaxVersions), list.Count)
}

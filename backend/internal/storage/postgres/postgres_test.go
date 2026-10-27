package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/pgtest"
	"gitlab.com/udevs/dbdiagram/backend/pkg/db"
)

// Repository suite: needs a real Postgres, so it is skipped with -short and
// when no test database answers (see pgtest).
type RepoSuite struct {
	suite.Suite
	pg *db.Postgres
	st storage.StorageI
	// me is the account every row below belongs to and every list runs as.
	me string
}

func TestRepoSuite(t *testing.T) {
	st, pg := pgtest.Open(t, "repo")
	suite.Run(t, &RepoSuite{st: st, pg: pg})
}

func (s *RepoSuite) SetupTest() {
	pgtest.Truncate(s.T(), s.pg)

	u, err := s.st.User().Create(context.Background(), &models.UserCreate{Email: "me@example.test", Name: "Me", PasswordHash: "x"})
	s.Require().NoError(err)
	s.me = u.Guid
}

func (s *RepoSuite) diagram(name string) *model.Diagram {
	d, err := s.st.Diagram().Create(context.Background(), &models.DiagramCreate{Name: name, Positions: map[string]model.Position{}, OwnerGuid: s.me})
	s.Require().NoError(err)

	return d
}

func (s *RepoSuite) kind(err error) apperr.Kind {
	s.Require().Error(err)
	_, e := apperr.HTTPStatus(err)

	return e.Kind
}

func (s *RepoSuite) find(p models.DiagramFind) *model.List[*model.DiagramItem] {
	p.ActorGuid = s.me
	res, err := s.st.Diagram().Find(context.Background(), &p)
	s.Require().NoError(err)

	return res
}

func names(l *model.List[*model.DiagramItem]) []string {
	out := []string{}
	for _, i := range l.Items {
		out = append(out, i.Name)
	}

	return out
}

func (s *RepoSuite) TestSoftDeleteHidesEverywhere() {
	ctx := context.Background()
	d := s.diagram("A")
	_, err := s.st.Diagram().SetShareToken(ctx, d.Guid, "tok")
	s.Require().NoError(err)

	s.Require().NoError(s.st.Diagram().Delete(ctx, d.Guid))

	_, err = s.st.Diagram().Get(ctx, d.Guid)
	s.Equal(apperr.KindNotFound, s.kind(err))
	s.Equal(int32(0), s.find(models.DiagramFind{}).Count)
	_, err = s.st.Diagram().GetShared(ctx, "tok")
	s.Equal(apperr.KindNotFound, s.kind(err), "the public link dies with the diagram")
	s.Equal(apperr.KindNotFound, s.kind(s.st.Diagram().Delete(ctx, d.Guid)), "deleting twice is NotFound")

	var rows int
	s.Require().NoError(s.pg.Db.Get(&rows, `SELECT count(1) FROM dbdiagram.diagrams WHERE guid = $1 AND deleted_at IS NOT NULL`, d.Guid))
	s.Equal(1, rows, "the row is kept, only stamped")
}

func (s *RepoSuite) TestUpdateWritesOnlyGivenFields() {
	ctx := context.Background()
	d, err := s.st.Diagram().Create(ctx, &models.DiagramCreate{Name: "A", DBML: "keep me", Positions: map[string]model.Position{"t": {X: 1, Y: 2}}, OwnerGuid: s.me})
	s.Require().NoError(err)
	time.Sleep(10 * time.Millisecond)

	name := "B"
	got, err := s.st.Diagram().Update(ctx, &models.DiagramPatch{Guid: d.Guid, Name: &name})
	s.Require().NoError(err)
	s.Equal("B", got.Name)
	s.Equal("keep me", got.DBML)
	s.Equal(model.Position{X: 1, Y: 2}, got.Positions["t"])

	// The API shows seconds; the trigger works at full resolution.
	var created, updated time.Time
	s.Require().NoError(s.pg.Db.QueryRow(`SELECT created_at, updated_at FROM dbdiagram.diagrams WHERE guid = $1`, d.Guid).Scan(&created, &updated))
	s.True(updated.After(created), "the trigger stamps updated_at")

	_, err = s.st.Diagram().Update(ctx, &models.DiagramPatch{Guid: d.Guid})
	s.Require().NoError(err)
	var untouched time.Time
	s.Require().NoError(s.pg.Db.QueryRow(`SELECT updated_at FROM dbdiagram.diagrams WHERE guid = $1`, d.Guid).Scan(&untouched))
	s.True(updated.Equal(untouched), "an empty patch writes nothing")

	_, err = s.st.Diagram().Update(ctx, &models.DiagramPatch{Guid: "00000000-0000-0000-0000-000000000000", Name: &name})
	s.Equal(apperr.KindNotFound, s.kind(err))

	_, err = s.st.Diagram().Get(ctx, "not-a-uuid")
	s.Equal(apperr.KindInvalidArgument, s.kind(err), "a malformed key is a 400, not a driver error")
}

func (s *RepoSuite) TestSearchTreatsWildcardsAsText() {
	s.diagram("100% done")
	s.diagram("a_b")
	s.diagram("axb")

	s.Equal([]string{"100% done"}, names(s.find(models.DiagramFind{Search: "%"})))
	s.Equal([]string{"a_b"}, names(s.find(models.DiagramFind{Search: "_"})))
	s.Equal(int32(1), s.find(models.DiagramFind{Search: "A_B"}).Count, "search is case-insensitive")
}

func (s *RepoSuite) TestFiltersAndSort() {
	ctx := context.Background()
	f, err := s.st.Folder().Create(ctx, &models.FolderCreate{Name: "F", OwnerGuid: s.me})
	s.Require().NoError(err)

	a, b, c := s.diagram("alpha"), s.diagram("Bravo"), s.diagram("charlie")
	_, err = s.st.Diagram().SetFolder(ctx, a.Guid, f.Guid)
	s.Require().NoError(err)

	eq := func(field, value string) models.DiagramFind {
		return models.DiagramFind{Filters: []model.Filter{{Field: field, Type: "eq", Value: value}}}
	}

	s.Equal([]string{"alpha"}, names(s.find(eq("folder_id", f.Guid))))
	s.ElementsMatch([]string{"Bravo", "charlie"}, names(s.find(eq("folder_id", ""))), "empty value = unfiled")
	s.ElementsMatch([]string{"Bravo", "charlie"}, names(s.find(models.DiagramFind{Filters: []model.Filter{{Field: "folder_id", Type: "neq", Value: f.Guid}}})), "neq keeps unfiled rows")
	s.Equal([]string{"Bravo"}, names(s.find(models.DiagramFind{Filters: []model.Filter{{Field: "name", Type: "search", Value: "rav"}}})))
	s.Equal(int32(3), s.find(models.DiagramFind{Filters: []model.Filter{{Field: "created_at", Type: "gte", Value: a.CreatedAt}}}).Count)
	s.Equal(int32(0), s.find(models.DiagramFind{Filters: []model.Filter{{Field: "updated_at", Type: "gt", Value: "2999-01-01T00:00:00Z"}}}).Count)
	_ = c

	for name, p := range map[string]models.DiagramFind{
		"unknown field": eq("dbml", "x"),
		"sql as field":  eq("1=1; --", "x"),
		"bad type":      {Filters: []model.Filter{{Field: "name", Type: "gt", Value: "x"}}},
		"bad uuid":      eq("folder_id", "nope"),
		"bad time":      {Filters: []model.Filter{{Field: "created_at", Type: "gt", Value: "yesterday"}}},
		"unknown sort":  {Sort: []model.SortBy{{Field: "dbml", Type: "asc"}}},
		"bad direction": {Sort: []model.SortBy{{Field: "name", Type: "sideways"}}},
		"sql as sort":   {Sort: []model.SortBy{{Field: "name; drop table diagrams", Type: "asc"}}},
	} {
		_, err := s.st.Diagram().Find(ctx, &p)
		s.Equal(apperr.KindInvalidArgument, s.kind(err), name)
	}

	asc := names(s.find(models.DiagramFind{Sort: []model.SortBy{{Field: "name", Type: "asc"}}}))
	s.Equal([]string{"alpha", "Bravo", "charlie"}, asc, "name sorts case-insensitively")
	desc := names(s.find(models.DiagramFind{Sort: []model.SortBy{{Field: "name", Type: "desc"}}}))
	s.Equal([]string{"charlie", "Bravo", "alpha"}, desc)
	_ = b
}

func (s *RepoSuite) TestPaginationIsStableWithEqualSortKeys() {
	for i := 0; i < 5; i++ {
		s.diagram("same")
	}

	seen := map[string]bool{}
	for page := int32(1); page <= 3; page++ {
		res := s.find(models.DiagramFind{Page: page, Limit: 2, Sort: []model.SortBy{{Field: "name", Type: "asc"}}})
		s.Equal(int32(5), res.Count, "count is the total, not the page")
		for _, i := range res.Items {
			s.False(seen[i.Guid], "a row must not appear on two pages")
			seen[i.Guid] = true
		}
	}
	s.Len(seen, 5)

	s.Empty(s.find(models.DiagramFind{Page: 4, Limit: 2}).Items)
	s.Len(s.find(models.DiagramFind{Limit: 100000}).Items, 5, "an absurd limit is clamped, not rejected")
}

func (s *RepoSuite) TestShareTokenIsUnique() {
	ctx := context.Background()
	a, b := s.diagram("A"), s.diagram("B")

	_, err := s.st.Diagram().SetShareToken(ctx, a.Guid, "same")
	s.Require().NoError(err)
	_, err = s.st.Diagram().SetShareToken(ctx, b.Guid, "same")
	s.Equal(apperr.KindAlreadyExists, s.kind(err))

	shared, err := s.st.Diagram().GetShared(ctx, "same")
	s.Require().NoError(err)
	s.Equal("A", shared.Name)

	_, err = s.st.Diagram().SetShareToken(ctx, a.Guid, "")
	s.Require().NoError(err)
	_, err = s.st.Diagram().SetShareToken(ctx, b.Guid, "same")
	s.NoError(err, "a revoked token can be reused")
}

func (s *RepoSuite) TestFolders() {
	ctx := context.Background()
	f, err := s.st.Folder().Create(ctx, &models.FolderCreate{Name: "Work", OwnerGuid: s.me})
	s.Require().NoError(err)
	s.Equal(s.me, f.OwnerID)
	s.Equal("Me", f.OwnerName)
	d := s.diagram("A")

	_, err = s.st.Diagram().SetFolder(ctx, d.Guid, "00000000-0000-0000-0000-000000000000")
	s.Equal(apperr.KindFailedPrecondition, s.kind(err), "the foreign key refuses a folder that does not exist")

	_, err = s.st.Diagram().SetFolder(ctx, d.Guid, f.Guid)
	s.Require().NoError(err)
	has, err := s.st.Folder().HasChildren(ctx, f.Guid)
	s.Require().NoError(err)
	s.True(has, "a diagram inside counts")

	_, err = s.st.Diagram().SetFolder(ctx, d.Guid, "")
	s.Require().NoError(err)
	has, err = s.st.Folder().HasChildren(ctx, f.Guid)
	s.Require().NoError(err)
	s.False(has)

	name := "Renamed"
	upd, err := s.st.Folder().Update(ctx, &models.FolderPatch{Guid: f.Guid, Name: &name})
	s.Require().NoError(err)
	s.Equal("Renamed", upd.Name)

	s.Require().NoError(s.st.Folder().Delete(ctx, f.Guid))
	_, err = s.st.Folder().Get(ctx, f.Guid)
	s.Equal(apperr.KindNotFound, s.kind(err))
	list, err := s.st.Folder().Find(ctx, &models.FolderFind{ActorGuid: s.me})
	s.Require().NoError(err)
	s.Zero(list.Count)
}

func (s *RepoSuite) TestTreeQueries() {
	ctx := context.Background()
	mk := func(name, parent string) string {
		f, err := s.st.Folder().Create(ctx, &models.FolderCreate{Name: name, ParentGuid: parent, OwnerGuid: s.me})
		s.Require().NoError(err)

		return f.Guid
	}

	// a > b > c, and a > d
	a := mk("a", "")
	b := mk("b", a)
	c := mk("c", b)
	d := mk("d", a)

	for guid, want := range map[string]int{a: 1, b: 2, c: 3, d: 2} {
		got, err := s.st.Folder().Depth(ctx, guid)
		s.Require().NoError(err)
		s.Equal(want, got, "depth")
	}
	for guid, want := range map[string]int{a: 3, b: 2, c: 1, d: 1} {
		got, err := s.st.Folder().Height(ctx, guid)
		s.Require().NoError(err)
		s.Equal(want, got, "height")
	}

	inside := func(guid, ancestor string) bool {
		got, err := s.st.Folder().IsInside(ctx, guid, ancestor)
		s.Require().NoError(err)

		return got
	}
	s.True(inside(c, a))
	s.True(inside(c, c), "a folder is inside itself")
	s.False(inside(a, c))
	s.False(inside(d, b), "siblings are not nested")

	// A deleted folder drops out of the chain.
	s.Require().NoError(s.st.Folder().Delete(ctx, c))
	h, err := s.st.Folder().Height(ctx, a)
	s.Require().NoError(err)
	s.Equal(2, h)

	// The list hides the parent link when the parent is out of reach.
	other, err := s.st.User().Create(ctx, &models.UserCreate{Email: "o@example.test", Name: "O", PasswordHash: "x"})
	s.Require().NoError(err)
	_, err = s.st.Member().SetFolder(ctx, b, other.Guid, model.RoleViewer)
	s.Require().NoError(err)
	list, err := s.st.Folder().Find(ctx, &models.FolderFind{ActorGuid: other.Guid})
	s.Require().NoError(err)
	s.Require().Len(list.Items, 1, "only b: a share reaches down, never up")
	s.Equal(b, list.Items[0].Guid)
	s.Empty(list.Items[0].ParentID, "b's parent a is invisible to them, so b looks top level")
	s.Equal(model.RoleViewer, list.Items[0].Role)
}

func (s *RepoSuite) TestSessionRevokeIsAtomic() {
	ctx := context.Background()
	s.Require().NoError(s.st.Session().Create(ctx, &models.SessionCreate{
		UserGuid: s.me, FamilyGuid: "11111111-1111-1111-1111-111111111111", TokenHash: "h1", ExpiresAt: time.Now().Add(time.Hour),
	}))

	sess, err := s.st.Session().GetByHash(ctx, "h1")
	s.Require().NoError(err)
	s.False(sess.Revoked)

	first, err := s.st.Session().Revoke(ctx, sess.Guid)
	s.Require().NoError(err)
	second, err := s.st.Session().Revoke(ctx, sess.Guid)
	s.Require().NoError(err)
	s.True(first, "the first caller wins the race")
	s.False(second, "the second is told it lost: that is what turns a replayed token into a reuse alarm")

	_, err = s.st.Session().GetByHash(ctx, "unknown")
	s.Equal(apperr.KindNotFound, s.kind(err))

	_, err = s.st.User().Create(ctx, &models.UserCreate{Email: "ME@example.test", Name: "Dup", PasswordHash: "x"})
	s.Equal(apperr.KindAlreadyExists, s.kind(err), "emails are unique ignoring case")
}

func (s *RepoSuite) TestTxRollsBackAndCommits() {
	ctx := context.Background()
	boom := errors.New("boom")

	err := s.st.Tx(ctx, func(st storage.StorageI) error {
		if _, err := st.Folder().Create(ctx, &models.FolderCreate{Name: "ghost", OwnerGuid: s.me}); err != nil {
			return err
		}

		return boom
	})
	s.ErrorIs(err, boom)

	list, err := s.st.Folder().Find(ctx, &models.FolderFind{ActorGuid: s.me})
	s.Require().NoError(err)
	s.Zero(list.Count, "a failed Tx leaves nothing behind")

	s.Require().NoError(s.st.Tx(ctx, func(st storage.StorageI) error {
		_, err := st.Folder().Create(ctx, &models.FolderCreate{Name: "kept", OwnerGuid: s.me})
		return err
	}))
	list, err = s.st.Folder().Find(ctx, &models.FolderFind{ActorGuid: s.me})
	s.Require().NoError(err)
	s.Equal(int32(1), list.Count)

	// Inside a Tx, nested Tx joins the outer one instead of committing early.
	err = s.st.Tx(ctx, func(st storage.StorageI) error {
		if err := st.Tx(ctx, func(inner storage.StorageI) error {
			_, err := inner.Folder().Create(ctx, &models.FolderCreate{Name: "nested", OwnerGuid: s.me})
			return err
		}); err != nil {
			return err
		}

		return boom
	})
	s.ErrorIs(err, boom)
	list, err = s.st.Folder().Find(ctx, &models.FolderFind{ActorGuid: s.me})
	s.Require().NoError(err)
	s.Equal(int32(1), list.Count, "the nested write rolled back with the outer Tx")
}

func (s *RepoSuite) version(diagramGuid, dbml string, at time.Time) *model.Version {
	v, err := s.st.Version().Create(context.Background(), &models.VersionCreate{
		DiagramGuid: diagramGuid, Label: "l", DBML: dbml, Positions: map[string]model.Position{"t": {X: 1, Y: 2}}, CreatedAt: at,
	})
	s.Require().NoError(err)

	return v
}

func (s *RepoSuite) TestVersionsAreNewestFirstAndPaged() {
	ctx := context.Background()
	d := s.diagram("A")
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Same second twice: only the guid tiebreak keeps the order stable across pages.
	s.version(d.Guid, "old", base)
	s.version(d.Guid, "tie-1", base.Add(time.Hour))
	s.version(d.Guid, "tie-2", base.Add(time.Hour))
	s.version(d.Guid, "new", base.Add(2*time.Hour))

	latest, err := s.st.Version().Latest(ctx, d.Guid)
	s.Require().NoError(err)
	s.Equal("new", latest.DBML)
	s.Equal(model.Position{X: 1, Y: 2}, latest.Positions["t"])

	seen := map[string]bool{}
	for page := int32(1); page <= 2; page++ {
		res, err := s.st.Version().Find(ctx, &models.VersionFind{DiagramGuid: d.Guid, Page: page, Limit: 2})
		s.Require().NoError(err)
		s.Equal(int32(4), res.Count)
		for _, i := range res.Items {
			s.False(seen[i.Guid], "a row must not appear on two pages")
			seen[i.Guid] = true
		}
	}
	s.Len(seen, 4)

	_, err = s.st.Version().Latest(ctx, s.diagram("empty").Guid)
	s.Equal(apperr.KindNotFound, s.kind(err))
}

func (s *RepoSuite) TestVersionsAreScopedAndDieWithTheirDiagram() {
	ctx := context.Background()
	a, b := s.diagram("A"), s.diagram("B")
	v := s.version(a.Guid, "a", time.Now())

	_, err := s.st.Version().Get(ctx, b.Guid, v.Guid)
	s.Equal(apperr.KindNotFound, s.kind(err), "a version is not reachable through another diagram's key")
	s.Equal(apperr.KindNotFound, s.kind(s.st.Version().Delete(ctx, b.Guid, v.Guid)))

	_, err = s.st.Version().Create(ctx, &models.VersionCreate{DiagramGuid: "00000000-0000-0000-0000-000000000000"})
	s.Equal(apperr.KindFailedPrecondition, s.kind(err), "the foreign key refuses a diagram that does not exist")

	s.Require().NoError(s.st.Diagram().Delete(ctx, a.Guid))
	_, err = s.st.Version().Get(ctx, a.Guid, v.Guid)
	s.Equal(apperr.KindNotFound, s.kind(err), "history is hidden with its diagram")
	list, err := s.st.Version().Find(ctx, &models.VersionFind{DiagramGuid: a.Guid})
	s.Require().NoError(err)
	s.Zero(list.Count)
}

func (s *RepoSuite) TestVersionPruneKeepsTheNewest() {
	ctx := context.Background()
	d, other := s.diagram("A"), s.diagram("B")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		s.version(d.Guid, fmt.Sprintf("v%d", i), base.Add(time.Duration(i)*time.Minute))
	}
	s.version(other.Guid, "untouched", base)

	s.Require().NoError(s.st.Version().Prune(ctx, d.Guid, 2))

	res, err := s.st.Version().Find(ctx, &models.VersionFind{DiagramGuid: d.Guid})
	s.Require().NoError(err)
	s.Equal(int32(2), res.Count)
	latest, _ := s.st.Version().Latest(ctx, d.Guid)
	s.Equal("v4", latest.DBML)

	o, err := s.st.Version().Find(ctx, &models.VersionFind{DiagramGuid: other.Guid})
	s.Require().NoError(err)
	s.Equal(int32(1), o.Count, "pruning one diagram never touches another")
}

func (s *RepoSuite) TestVersionCreateIsPartOfTheDiagramTransaction() {
	ctx := context.Background()
	d := s.diagram("A")
	boom := errors.New("boom")

	err := s.st.Tx(ctx, func(st storage.StorageI) error {
		if _, err := st.Version().Create(ctx, &models.VersionCreate{DiagramGuid: d.Guid, DBML: "x"}); err != nil {
			return err
		}

		return boom
	})
	s.ErrorIs(err, boom)

	res, err := s.st.Version().Find(ctx, &models.VersionFind{DiagramGuid: d.Guid})
	s.Require().NoError(err)
	s.Zero(res.Count, "a snapshot rolls back with the save it belongs to")
}

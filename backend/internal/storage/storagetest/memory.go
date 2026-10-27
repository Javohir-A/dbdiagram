// Package storagetest is an in-memory storage.StorageI for service and handler
// tests. It follows the contract of the Postgres repositories (soft delete,
// NotFound on a missing row, owners, folder trees, shares, role inheritance)
// without any SQL. List filters and sorting are NOT simulated: they are
// covered by the Postgres suite. The access rules ARE simulated, and
// authz_test.go runs the same scenarios against this fake and against Postgres
// so the two cannot drift apart.
package storagetest

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/repo"
)

type userRow struct {
	user model.User
	hash string
}

type sessionRow struct {
	guid, user, family, hash string
	expires                  time.Time
	revoked                  bool
}

type Memory struct {
	mu       sync.Mutex
	users    map[string]*userRow
	sessions []*sessionRow
	diagrams map[string]*model.Diagram
	folders  map[string]*model.Folder
	fshares  map[string]map[string]model.Role // folder -> user -> role
	dshares  map[string]map[string]model.Role // diagram -> user -> role
	versions []*model.Version                 // insertion order == creation order
	tokens   []*tokenRow
}

func New() *Memory {
	return &Memory{
		users:    map[string]*userRow{},
		diagrams: map[string]*model.Diagram{},
		folders:  map[string]*model.Folder{},
		fshares:  map[string]map[string]model.Role{},
		dshares:  map[string]map[string]model.Role{},
	}
}

func (m *Memory) Access() repo.AccessI   { return &accessRepo{m} }
func (m *Memory) Diagram() repo.DiagramI { return &diagramRepo{m} }
func (m *Memory) Folder() repo.FolderI   { return &folderRepo{m} }
func (m *Memory) Member() repo.MemberI   { return &memberRepo{m} }
func (m *Memory) Session() repo.SessionI { return &sessionRepo{m} }
func (m *Memory) Token() repo.TokenI     { return &tokenRepo{m} }
func (m *Memory) User() repo.UserI       { return &userRepo{m} }
func (m *Memory) Version() repo.VersionI { return &versionRepo{m} }

// Tx has no rollback: the fakes are only used for success and validation paths.
func (m *Memory) Tx(_ context.Context, fn func(storage.StorageI) error) error { return fn(m) }

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// ---- access ----------------------------------------------------------------

// folderRank is the highest of owning the folder or an ancestor and a share on
// the folder or an ancestor: the same rule the SQL implements. Caller holds mu.
func (m *Memory) folderRank(user, folder string) int {
	best := 0

	for cur, hops := folder, 0; cur != "" && hops < 50; hops++ {
		f, ok := m.folders[cur]
		if !ok {
			break
		}
		if f.OwnerID == user {
			best = 3
		}
		if r, ok := m.fshares[cur][user]; ok && r.Rank() > best {
			best = r.Rank()
		}
		cur = f.ParentID
	}

	return best
}

func (m *Memory) diagramRank(user string, d *model.Diagram) int {
	best := m.folderRank(user, d.FolderID)

	if d.OwnerID == user {
		best = 3
	}
	if r, ok := m.dshares[d.Guid][user]; ok && r.Rank() > best {
		best = r.Rank()
	}

	return best
}

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

type accessRepo struct{ m *Memory }

func (r *accessRepo) FolderRole(_ context.Context, user, folder string) (model.Role, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	return roleOf(r.m.folderRank(user, folder)), nil
}

func (r *accessRepo) DiagramRole(_ context.Context, user, diagram string) (model.Role, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	d, ok := r.m.diagrams[diagram]
	if !ok {
		return model.RoleNone, nil
	}

	return roleOf(r.m.diagramRank(user, d)), nil
}

// ---- users and sessions ----------------------------------------------------

type userRepo struct{ m *Memory }

func (r *userRepo) Create(_ context.Context, p *models.UserCreate) (*model.User, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, u := range r.m.users {
		if strings.EqualFold(u.user.Email, p.Email) {
			return nil, apperr.AlreadyExists("already exists")
		}
	}
	u := &userRow{user: model.User{Guid: uuid.NewString(), Email: p.Email, Name: p.Name, CreatedAt: now()}, hash: p.PasswordHash}
	r.m.users[u.user.Guid] = u
	c := u.user

	return &c, nil
}

func (r *userRepo) Get(_ context.Context, guid string) (*model.User, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	u, ok := r.m.users[guid]
	if !ok {
		return nil, apperr.NotFound("this information does not exist")
	}
	c := u.user

	return &c, nil
}

func (r *userRepo) byEmail(email string) *userRow {
	for _, u := range r.m.users {
		if strings.EqualFold(u.user.Email, email) {
			return u
		}
	}

	return nil
}

func (r *userRepo) GetByEmail(_ context.Context, email string) (*model.User, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	u := r.byEmail(email)
	if u == nil {
		return nil, apperr.NotFound("this information does not exist")
	}
	c := u.user

	return &c, nil
}

func (r *userRepo) GetCredentials(_ context.Context, email string) (*models.Credentials, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	u := r.byEmail(email)
	if u == nil {
		return nil, apperr.NotFound("this information does not exist")
	}

	return &models.Credentials{User: u.user, PasswordHash: u.hash}, nil
}

func (r *userRepo) GetPasswordHash(_ context.Context, guid string) (string, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	u, ok := r.m.users[guid]
	if !ok {
		return "", apperr.NotFound("this information does not exist")
	}

	return u.hash, nil
}

func (r *userRepo) UpdateName(_ context.Context, guid, name string) (*model.User, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	u, ok := r.m.users[guid]
	if !ok {
		return nil, apperr.NotFound("user not found")
	}
	u.user.Name = name
	c := u.user

	return &c, nil
}

func (r *userRepo) UpdatePassword(_ context.Context, guid, hash string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	u, ok := r.m.users[guid]
	if !ok {
		return apperr.NotFound("user not found")
	}
	u.hash = hash

	return nil
}

type sessionRepo struct{ m *Memory }

func (r *sessionRepo) Create(_ context.Context, p *models.SessionCreate) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	r.m.sessions = append(r.m.sessions, &sessionRow{guid: uuid.NewString(), user: p.UserGuid, family: p.FamilyGuid, hash: p.TokenHash, expires: p.ExpiresAt})

	return nil
}

func (r *sessionRepo) GetByHash(_ context.Context, hash string) (*models.Session, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, s := range r.m.sessions {
		if s.hash == hash {
			return &models.Session{Guid: s.guid, UserGuid: s.user, FamilyGuid: s.family, ExpiresAt: s.expires, Revoked: s.revoked}, nil
		}
	}

	return nil, apperr.NotFound("this information does not exist")
}

func (r *sessionRepo) Revoke(_ context.Context, guid string) (bool, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, s := range r.m.sessions {
		if s.guid == guid && !s.revoked {
			s.revoked = true
			return true, nil
		}
	}

	return false, nil
}

func (r *sessionRepo) RevokeFamily(_ context.Context, family string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, s := range r.m.sessions {
		if s.family == family {
			s.revoked = true
		}
	}

	return nil
}

func (r *sessionRepo) RevokeAllForUser(_ context.Context, user, except string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, s := range r.m.sessions {
		if s.user == user && (except == "" || s.family != except) {
			s.revoked = true
		}
	}

	return nil
}

// ---- diagrams --------------------------------------------------------------

type diagramRepo struct{ m *Memory }

func (r *diagramRepo) ownerName(id string) string {
	if u, ok := r.m.users[id]; ok {
		return u.user.Name
	}

	return ""
}

func (r *diagramRepo) Create(_ context.Context, p *models.DiagramCreate) (*model.Diagram, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	d := &model.Diagram{
		Guid: uuid.NewString(), Name: p.Name, DBML: p.DBML, Positions: p.Positions, FolderID: p.FolderGuid,
		OwnerID: p.OwnerGuid, Revision: 1, UpdatedByName: r.ownerName(p.OwnerGuid), CreatedAt: now(), UpdatedAt: now(),
	}
	d.OwnerName = r.ownerName(d.OwnerID)
	r.m.diagrams[d.Guid] = d
	c := *d

	return &c, nil
}

func (r *diagramRepo) live(guid string) (*model.Diagram, error) {
	d, ok := r.m.diagrams[guid]
	if !ok {
		return nil, apperr.NotFound("diagram not found")
	}

	return d, nil
}

func (r *diagramRepo) Get(_ context.Context, guid string) (*model.Diagram, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	d, err := r.live(guid)
	if err != nil {
		return nil, err
	}
	c := *d

	return &c, nil
}

func (r *diagramRepo) Revision(_ context.Context, guid string) (*model.Revision, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	d, err := r.live(guid)
	if err != nil {
		return nil, err
	}

	return &model.Revision{Revision: d.Revision, UpdatedAt: d.UpdatedAt, UpdatedByName: d.UpdatedByName}, nil
}

func (r *diagramRepo) GetShared(_ context.Context, token string) (*model.SharedDiagram, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, d := range r.m.diagrams {
		if d.ShareToken != "" && d.ShareToken == token {
			return &model.SharedDiagram{Name: d.Name, DBML: d.DBML, Positions: d.Positions}, nil
		}
	}

	return nil, apperr.NotFound("this information does not exist")
}

func (r *diagramRepo) Find(_ context.Context, p *models.DiagramFind) (*model.List[*model.DiagramItem], error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	res := &model.List[*model.DiagramItem]{Items: []*model.DiagramItem{}}
	for _, d := range r.m.diagrams {
		rank := r.m.diagramRank(p.ActorGuid, d)
		if rank == 0 {
			continue
		}
		if p.Search != "" && !strings.Contains(strings.ToLower(d.Name), strings.ToLower(p.Search)) {
			continue
		}

		item := &model.DiagramItem{Guid: d.Guid, Name: d.Name, FolderID: d.FolderID, OwnerID: d.OwnerID, OwnerName: d.OwnerName, Role: roleOf(rank), CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
		if r.m.folderRank(p.ActorGuid, d.FolderID) == 0 {
			item.FolderID = ""
		}
		if rank == 3 {
			item.ShareToken = d.ShareToken
		}
		res.Items = append(res.Items, item)
	}
	sort.Slice(res.Items, func(i, j int) bool { return res.Items[i].Guid < res.Items[j].Guid })
	res.Count = int32(len(res.Items))

	return res, nil
}

func (r *diagramRepo) Update(_ context.Context, patch *models.DiagramPatch) (*model.Diagram, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	d, err := r.live(patch.Guid)
	if err != nil {
		return nil, err
	}
	if patch.DBML != nil && patch.BaseRevision != nil && *patch.BaseRevision != d.Revision {
		return nil, apperr.RevisionConflict("this diagram was changed by someone else since you loaded it")
	}

	if patch.Name != nil {
		d.Name = *patch.Name
	}
	if patch.DBML != nil {
		if d.DBML != *patch.DBML {
			d.Revision++
		}
		d.DBML = *patch.DBML
	}
	if patch.Positions != nil {
		d.Positions = *patch.Positions
	}
	d.UpdatedByName = r.ownerName(patch.UpdatedBy)
	d.UpdatedAt = now()
	c := *d

	return &c, nil
}

func (r *diagramRepo) SetFolder(_ context.Context, guid, folderGuid string) (*model.Diagram, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	d, err := r.live(guid)
	if err != nil {
		return nil, err
	}
	d.FolderID = folderGuid
	c := *d

	return &c, nil
}

func (r *diagramRepo) SetShareToken(_ context.Context, guid, token string) (*model.Diagram, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	d, err := r.live(guid)
	if err != nil {
		return nil, err
	}
	d.ShareToken = token
	c := *d

	return &c, nil
}

func (r *diagramRepo) Delete(_ context.Context, guid string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	if _, err := r.live(guid); err != nil {
		return err
	}
	delete(r.m.diagrams, guid)

	return nil
}

func (r *diagramRepo) ClaimUnowned(_ context.Context, owner string) (int64, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	var n int64
	for _, d := range r.m.diagrams {
		if d.OwnerID == "" {
			d.OwnerID, d.OwnerName = owner, r.ownerName(owner)
			n++
		}
	}

	return n, nil
}

// ---- folders ---------------------------------------------------------------

type folderRepo struct{ m *Memory }

func (r *folderRepo) ownerName(id string) string {
	if u, ok := r.m.users[id]; ok {
		return u.user.Name
	}

	return ""
}

func (r *folderRepo) Create(_ context.Context, p *models.FolderCreate) (*model.Folder, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	f := &model.Folder{Guid: uuid.NewString(), Name: p.Name, ParentID: p.ParentGuid, OwnerID: p.OwnerGuid, OwnerName: r.ownerName(p.OwnerGuid), CreatedAt: now(), UpdatedAt: now()}
	r.m.folders[f.Guid] = f
	c := *f

	return &c, nil
}

func (r *folderRepo) Get(_ context.Context, guid string) (*model.Folder, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	f, ok := r.m.folders[guid]
	if !ok {
		return nil, apperr.NotFound("folder not found")
	}
	c := *f

	return &c, nil
}

func (r *folderRepo) Find(_ context.Context, p *models.FolderFind) (*model.List[*model.Folder], error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	res := &model.List[*model.Folder]{Items: []*model.Folder{}}
	for _, f := range r.m.folders {
		rank := r.m.folderRank(p.ActorGuid, f.Guid)
		if rank == 0 {
			continue
		}
		c := *f
		c.Role = roleOf(rank)
		if r.m.folderRank(p.ActorGuid, f.ParentID) == 0 {
			c.ParentID = ""
		}
		res.Items = append(res.Items, &c)
	}
	sort.Slice(res.Items, func(i, j int) bool { return res.Items[i].Name < res.Items[j].Name })
	res.Count = int32(len(res.Items))

	return res, nil
}

func (r *folderRepo) Update(_ context.Context, patch *models.FolderPatch) (*model.Folder, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	f, ok := r.m.folders[patch.Guid]
	if !ok {
		return nil, apperr.NotFound("folder not found")
	}
	if patch.Name != nil {
		f.Name = *patch.Name
	}
	c := *f

	return &c, nil
}

func (r *folderRepo) SetParent(_ context.Context, guid, parent string) (*model.Folder, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	f, ok := r.m.folders[guid]
	if !ok {
		return nil, apperr.NotFound("folder not found")
	}
	f.ParentID = parent
	c := *f

	return &c, nil
}

func (r *folderRepo) Delete(_ context.Context, guid string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	if _, ok := r.m.folders[guid]; !ok {
		return apperr.NotFound("folder not found")
	}
	delete(r.m.folders, guid)

	return nil
}

func (r *folderRepo) HasChildren(_ context.Context, guid string) (bool, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, f := range r.m.folders {
		if f.ParentID == guid {
			return true, nil
		}
	}
	for _, d := range r.m.diagrams {
		if d.FolderID == guid {
			return true, nil
		}
	}

	return false, nil
}

func (r *folderRepo) Depth(_ context.Context, guid string) (int, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	n := 0
	for cur := guid; cur != "" && n < 50; n++ {
		f, ok := r.m.folders[cur]
		if !ok {
			break
		}
		cur = f.ParentID
	}

	return n, nil
}

func (r *folderRepo) height(guid string, n int) int {
	if _, ok := r.m.folders[guid]; !ok || n > 50 {
		return 0
	}

	best := 1
	for _, f := range r.m.folders {
		if f.ParentID == guid {
			if h := 1 + r.height(f.Guid, n+1); h > best {
				best = h
			}
		}
	}

	return best
}

func (r *folderRepo) Height(_ context.Context, guid string) (int, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	return r.height(guid, 0), nil
}

func (r *folderRepo) IsInside(_ context.Context, guid, ancestor string) (bool, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for cur, hops := guid, 0; cur != "" && hops < 50; hops++ {
		if cur == ancestor {
			return true, nil
		}
		f, ok := r.m.folders[cur]
		if !ok {
			break
		}
		cur = f.ParentID
	}

	return false, nil
}

func (r *folderRepo) ClaimUnowned(_ context.Context, owner string) (int64, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	var n int64
	for _, f := range r.m.folders {
		if f.OwnerID == "" {
			f.OwnerID, f.OwnerName = owner, r.ownerName(owner)
			n++
		}
	}

	return n, nil
}

// ---- members ---------------------------------------------------------------

type memberRepo struct{ m *Memory }

func (r *memberRepo) list(shares map[string]map[string]model.Role, guid string) []*model.Member {
	out := []*model.Member{}
	for user, role := range shares[guid] {
		u := r.m.users[user]
		out = append(out, &model.Member{UserID: user, Email: u.user.Email, Name: u.user.Name, Role: role, CreatedAt: now()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })

	return out
}

func (r *memberRepo) set(shares map[string]map[string]model.Role, guid, user string, role model.Role) (*model.Member, error) {
	u, ok := r.m.users[user]
	if !ok {
		return nil, apperr.FailedPrecondition("the elements related to this one must be removed first")
	}
	if shares[guid] == nil {
		shares[guid] = map[string]model.Role{}
	}
	shares[guid][user] = role

	return &model.Member{UserID: user, Email: u.user.Email, Name: u.user.Name, Role: role, CreatedAt: now()}, nil
}

func (r *memberRepo) remove(shares map[string]map[string]model.Role, guid, user string) error {
	if _, ok := shares[guid][user]; !ok {
		return apperr.NotFound("this person has no direct access here")
	}
	delete(shares[guid], user)

	return nil
}

func (r *memberRepo) ListFolder(_ context.Context, g string) ([]*model.Member, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	return r.list(r.m.fshares, g), nil
}

func (r *memberRepo) ListDiagram(_ context.Context, g string) ([]*model.Member, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	return r.list(r.m.dshares, g), nil
}

func (r *memberRepo) SetFolder(_ context.Context, g, u string, role model.Role) (*model.Member, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	return r.set(r.m.fshares, g, u, role)
}

func (r *memberRepo) SetDiagram(_ context.Context, g, u string, role model.Role) (*model.Member, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	return r.set(r.m.dshares, g, u, role)
}

func (r *memberRepo) RemoveFolder(_ context.Context, g, u string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	return r.remove(r.m.fshares, g, u)
}

func (r *memberRepo) RemoveDiagram(_ context.Context, g, u string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	return r.remove(r.m.dshares, g, u)
}

// ---- versions --------------------------------------------------------------

type versionRepo struct{ m *Memory }

// forDiagram is newest first, like the SQL ORDER BY.
func (r *versionRepo) forDiagram(diagramGuid string) []*model.Version {
	var out []*model.Version
	if _, ok := r.m.diagrams[diagramGuid]; !ok {
		return nil
	}
	for i := len(r.m.versions) - 1; i >= 0; i-- {
		if r.m.versions[i].DiagramID == diagramGuid {
			out = append(out, r.m.versions[i])
		}
	}

	return out
}

func (r *versionRepo) Create(_ context.Context, p *models.VersionCreate) (*model.Version, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	at := p.CreatedAt
	if at.IsZero() {
		at = time.Now()
	}
	v := &model.Version{Guid: uuid.NewString(), DiagramID: p.DiagramGuid, Label: p.Label, DBML: p.DBML, Positions: p.Positions, CreatedAt: at.UTC().Format(time.RFC3339)}
	r.m.versions = append(r.m.versions, v)
	c := *v

	return &c, nil
}

func (r *versionRepo) Get(_ context.Context, diagramGuid, guid string) (*model.Version, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, v := range r.forDiagram(diagramGuid) {
		if v.Guid == guid {
			c := *v
			return &c, nil
		}
	}

	return nil, apperr.NotFound("this information does not exist")
}

func (r *versionRepo) Latest(_ context.Context, diagramGuid string) (*model.Version, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	all := r.forDiagram(diagramGuid)
	if len(all) == 0 {
		return nil, apperr.NotFound("this information does not exist")
	}
	c := *all[0]

	return &c, nil
}

func (r *versionRepo) Find(_ context.Context, p *models.VersionFind) (*model.List[*model.VersionItem], error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	res := &model.List[*model.VersionItem]{Items: []*model.VersionItem{}}
	all := r.forDiagram(p.DiagramGuid)
	res.Count = int32(len(all))
	for _, v := range all {
		res.Items = append(res.Items, &model.VersionItem{Guid: v.Guid, DiagramID: v.DiagramID, Label: v.Label, CreatedAt: v.CreatedAt})
	}

	return res, nil
}

func (r *versionRepo) Delete(_ context.Context, diagramGuid, guid string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for i, v := range r.m.versions {
		if v.Guid == guid && v.DiagramID == diagramGuid {
			r.m.versions = append(r.m.versions[:i], r.m.versions[i+1:]...)
			return nil
		}
	}

	return apperr.NotFound("version not found")
}

func (r *versionRepo) Prune(_ context.Context, diagramGuid string, keep int) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	drop := map[string]bool{}
	for i, v := range r.forDiagram(diagramGuid) {
		if i >= keep {
			drop[v.Guid] = true
		}
	}
	kept := r.m.versions[:0]
	for _, v := range r.m.versions {
		if !drop[v.Guid] {
			kept = append(kept, v)
		}
	}
	r.m.versions = kept

	return nil
}

// ---- access tokens ---------------------------------------------------------

type tokenRow struct {
	model.AccessToken
	user, hash string
	expires    time.Time
	lastUsed   time.Time
	revoked    bool
}

type tokenRepo struct{ m *Memory }

func (r *tokenRepo) show(t *tokenRow) *model.AccessToken {
	c := t.AccessToken
	c.ExpiresAt = t.expires.UTC().Format(time.RFC3339)
	if !t.lastUsed.IsZero() {
		c.LastUsedAt = t.lastUsed.UTC().Format(time.RFC3339)
	}

	return &c
}

func (r *tokenRepo) Create(_ context.Context, p *models.TokenCreate) (*model.AccessToken, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	t := &tokenRow{
		AccessToken: model.AccessToken{Guid: uuid.NewString(), Name: p.Name, Prefix: p.Prefix, Scope: p.Scope, CreatedAt: now()},
		user:        p.UserGuid, hash: p.TokenHash, expires: p.ExpiresAt,
	}
	r.m.tokens = append(r.m.tokens, t)

	return r.show(t), nil
}

func (r *tokenRepo) usable(t *tokenRow, at time.Time) bool { return !t.revoked && t.expires.After(at) }

func (r *tokenRepo) ListActive(_ context.Context, user string, at time.Time) ([]*model.AccessToken, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	out := []*model.AccessToken{}
	for i := len(r.m.tokens) - 1; i >= 0; i-- {
		if t := r.m.tokens[i]; t.user == user && r.usable(t, at) {
			out = append(out, r.show(t))
		}
	}

	return out, nil
}

func (r *tokenRepo) CountActive(ctx context.Context, user string, at time.Time) (int, error) {
	list, _ := r.ListActive(ctx, user, at)

	return len(list), nil
}

func (r *tokenRepo) GetActiveByHash(_ context.Context, hash string, at time.Time) (*models.TokenAuth, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, t := range r.m.tokens {
		if t.hash == hash && r.usable(t, at) {
			if _, ok := r.m.users[t.user]; ok {
				return &models.TokenAuth{Guid: t.Guid, UserGuid: t.user, Scope: t.Scope, LastUsedAt: t.lastUsed}, nil
			}
		}
	}

	return nil, apperr.NotFound("this information does not exist")
}

func (r *tokenRepo) Touch(_ context.Context, guid string, at, before time.Time) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, t := range r.m.tokens {
		if t.Guid == guid && (t.lastUsed.IsZero() || t.lastUsed.Before(before)) {
			t.lastUsed = at
		}
	}

	return nil
}

func (r *tokenRepo) Revoke(_ context.Context, user, guid string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()

	for _, t := range r.m.tokens {
		if t.Guid == guid && t.user == user && !t.revoked {
			t.revoked = true
			return nil
		}
	}

	return apperr.NotFound("token not found")
}

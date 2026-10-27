package diagrams

import (
	"context"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
)

// MemberService manages who a folder or a diagram is shared with. A share on a
// folder reaches everything inside it; the owner and ancestors' owners are never
// rows, they hold access by owning.
type MemberService struct {
	deps Deps
}

// resource abstracts the two kinds so folders and diagrams share one flow.
type resource struct {
	require func(ctx context.Context, st storage.StorageI, actor, guid string, need model.Role) (model.Role, error)
	owner   func(ctx context.Context, st storage.StorageI, guid string) (string, error)
	list    func(ctx context.Context, st storage.StorageI, guid string) ([]*model.Member, error)
	set     func(ctx context.Context, st storage.StorageI, guid, userGuid string, role model.Role) (*model.Member, error)
	remove  func(ctx context.Context, st storage.StorageI, guid, userGuid string) error
}

var folderResource = resource{
	require: requireFolder,
	owner: func(ctx context.Context, st storage.StorageI, guid string) (string, error) {
		f, err := st.Folder().Get(ctx, guid)
		if err != nil {
			return "", err
		}

		return f.OwnerID, nil
	},
	list: func(ctx context.Context, st storage.StorageI, guid string) ([]*model.Member, error) {
		return st.Member().ListFolder(ctx, guid)
	},
	set: func(ctx context.Context, st storage.StorageI, guid, userGuid string, role model.Role) (*model.Member, error) {
		return st.Member().SetFolder(ctx, guid, userGuid, role)
	},
	remove: func(ctx context.Context, st storage.StorageI, guid, userGuid string) error {
		return st.Member().RemoveFolder(ctx, guid, userGuid)
	},
}

var diagramResource = resource{
	require: requireDiagram,
	owner: func(ctx context.Context, st storage.StorageI, guid string) (string, error) {
		d, err := st.Diagram().Get(ctx, guid)
		if err != nil {
			return "", err
		}

		return d.OwnerID, nil
	},
	list: func(ctx context.Context, st storage.StorageI, guid string) ([]*model.Member, error) {
		return st.Member().ListDiagram(ctx, guid)
	},
	set: func(ctx context.Context, st storage.StorageI, guid, userGuid string, role model.Role) (*model.Member, error) {
		return st.Member().SetDiagram(ctx, guid, userGuid, role)
	},
	remove: func(ctx context.Context, st storage.StorageI, guid, userGuid string) error {
		return st.Member().RemoveDiagram(ctx, guid, userGuid)
	},
}

func (s *MemberService) ListFolder(ctx context.Context, actor, guid string) ([]*model.Member, error) {
	return s.list(ctx, folderResource, actor, guid)
}

func (s *MemberService) ListDiagram(ctx context.Context, actor, guid string) ([]*model.Member, error) {
	return s.list(ctx, diagramResource, actor, guid)
}

func (s *MemberService) SetFolder(ctx context.Context, actor, guid, email string, role model.Role) (*model.Member, error) {
	return s.set(ctx, folderResource, actor, guid, email, role)
}

func (s *MemberService) SetDiagram(ctx context.Context, actor, guid, email string, role model.Role) (*model.Member, error) {
	return s.set(ctx, diagramResource, actor, guid, email, role)
}

func (s *MemberService) RemoveFolder(ctx context.Context, actor, guid, userGuid string) error {
	return s.remove(ctx, folderResource, actor, guid, userGuid)
}

func (s *MemberService) RemoveDiagram(ctx context.Context, actor, guid, userGuid string) error {
	return s.remove(ctx, diagramResource, actor, guid, userGuid)
}

// list shows the owner first, then the direct shares. Owner-rank callers only:
// who else can see something is not for everyone to know.
func (s *MemberService) list(ctx context.Context, r resource, actor, guid string) ([]*model.Member, error) {
	st := s.deps.Storage

	if _, err := r.require(ctx, st, actor, guid, model.RoleOwner); err != nil {
		return nil, err
	}
	ownerID, err := r.owner(ctx, st, guid)
	if err != nil {
		return nil, err
	}
	shares, err := r.list(ctx, st, guid)
	if err != nil {
		return nil, err
	}

	out := make([]*model.Member, 0, len(shares)+1)
	if ownerID != "" {
		u, err := s.deps.Users.ByID(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		out = append(out, &model.Member{UserID: u.Guid, Email: u.Email, Name: u.Name, Role: model.RoleOwner, CreatedAt: u.CreatedAt})
	}

	return append(out, shares...), nil
}

func (s *MemberService) set(ctx context.Context, r resource, actor, guid, email string, role model.Role) (*model.Member, error) {
	if !role.Shareable() {
		return nil, apperr.InvalidArgument("role must be editor or viewer")
	}

	var res *model.Member

	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		if _, err := r.require(ctx, st, actor, guid, model.RoleOwner); err != nil {
			return err
		}

		u, err := s.deps.Users.ByEmail(ctx, email)
		if isNotFound(err) {
			return apperr.NotFound("no user with that email")
		}
		if err != nil {
			return err
		}

		ownerID, err := r.owner(ctx, st, guid)
		if err != nil {
			return err
		}
		if u.Guid == actor || u.Guid == ownerID {
			return apperr.InvalidArgument("that person already has full access")
		}

		res, err = r.set(ctx, st, guid, u.Guid, role)

		return err
	})

	return res, err
}

// remove ends a direct share. The owner-rank can remove anyone's; anyone can
// remove their OWN (leave). Leaving needs no role check on purpose: it only ever
// deletes the caller's own row, and answers NotFound when there is none.
func (s *MemberService) remove(ctx context.Context, r resource, actor, guid, userGuid string) error {
	return s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		if userGuid != actor {
			if _, err := r.require(ctx, st, actor, guid, model.RoleOwner); err != nil {
				return err
			}
		}

		return r.remove(ctx, st, guid, userGuid)
	})
}

package diagrams

import (
	"context"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage/models"
)

type FolderService struct {
	deps Deps
}

type FindFoldersParams struct {
	Search string
	Sort   []model.SortBy
	Page   int32
	Limit  int32
}

// Create makes a folder owned by the actor, optionally inside another folder
// they may edit.
func (s *FolderService) Create(ctx context.Context, actor, name, parentID string) (*model.Folder, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}

	var res *model.Folder

	err = s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		if parentID != "" {
			if err := requireParent(ctx, st, actor, parentID); err != nil {
				return err
			}

			depth, err := st.Folder().Depth(ctx, parentID)
			if err != nil {
				return err
			}
			if depth+1 > config.MaxFolderDepth {
				return apperr.InvalidArgument("folders can be nested at most %d levels deep", config.MaxFolderDepth)
			}
		}

		f, err := st.Folder().Create(ctx, &models.FolderCreate{Name: name, ParentGuid: parentID, OwnerGuid: actor})
		if err != nil {
			return err
		}

		res, err = presentFolder(ctx, st, actor, f, model.RoleOwner)

		return err
	})

	return res, err
}

func (s *FolderService) Get(ctx context.Context, actor, guid string) (*model.Folder, error) {
	st := s.deps.Storage

	role, err := requireFolder(ctx, st, actor, guid, model.RoleViewer)
	if err != nil {
		return nil, err
	}
	f, err := st.Folder().Get(ctx, guid)
	if err != nil {
		return nil, err
	}

	return presentFolder(ctx, st, actor, f, role)
}

func (s *FolderService) Find(ctx context.Context, actor string, p FindFoldersParams) (*model.List[*model.Folder], error) {
	return s.deps.Storage.Folder().Find(ctx, &models.FolderFind{
		ActorGuid: actor, Search: p.Search, Sort: p.Sort, Page: p.Page, Limit: p.Limit,
	})
}

// Update serves PUT and PATCH: a nil name keeps the stored one.
func (s *FolderService) Update(ctx context.Context, actor, guid string, name *string) (*model.Folder, error) {
	patch := &models.FolderPatch{Guid: guid}

	if name != nil {
		clean, err := cleanName(*name)
		if err != nil {
			return nil, err
		}
		patch.Name = &clean
	}

	var res *model.Folder

	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		role, err := requireFolder(ctx, st, actor, guid, model.RoleEditor)
		if err != nil {
			return err
		}
		f, err := st.Folder().Update(ctx, patch)
		if err != nil {
			return err
		}

		res, err = presentFolder(ctx, st, actor, f, role)

		return err
	})

	return res, err
}

// SetParent moves a folder: owner on it, editor on the destination, never into
// itself or its own subtree, and never deeper than config.MaxFolderDepth.
func (s *FolderService) SetParent(ctx context.Context, actor, guid, parentID string) (*model.Folder, error) {
	var res *model.Folder

	err := s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		role, err := requireFolder(ctx, st, actor, guid, model.RoleOwner)
		if err != nil {
			return err
		}

		if parentID != "" {
			if err := requireParent(ctx, st, actor, parentID); err != nil {
				return err
			}

			inside, err := st.Folder().IsInside(ctx, parentID, guid)
			if err != nil {
				return err
			}
			if inside {
				return apperr.InvalidArgument("a folder cannot be moved into itself or its own subfolders")
			}

			depth, err := st.Folder().Depth(ctx, parentID)
			if err != nil {
				return err
			}
			height, err := st.Folder().Height(ctx, guid)
			if err != nil {
				return err
			}
			if depth+height > config.MaxFolderDepth {
				return apperr.InvalidArgument("folders can be nested at most %d levels deep", config.MaxFolderDepth)
			}
		}

		f, err := st.Folder().SetParent(ctx, guid, parentID)
		if err != nil {
			return err
		}

		res, err = presentFolder(ctx, st, actor, f, role)

		return err
	})

	return res, err
}

// Delete removes an empty folder (owner only). A folder that still holds
// anything is refused: silently moving or hiding its contents could change who
// can see them.
func (s *FolderService) Delete(ctx context.Context, actor, guid string) error {
	return s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		if _, err := requireFolder(ctx, st, actor, guid, model.RoleOwner); err != nil {
			return err
		}

		has, err := st.Folder().HasChildren(ctx, guid)
		if err != nil {
			return err
		}
		if has {
			return apperr.FailedPrecondition("folder is not empty: move or delete its contents first")
		}

		return st.Folder().Delete(ctx, guid)
	})
}

// requireParent checks the folder something nests under: like requireDestination,
// but a bad parent_id is the caller's argument.
func requireParent(ctx context.Context, st storage.StorageI, actor, parentID string) error {
	_, err := requireFolder(ctx, st, actor, parentID, model.RoleEditor)
	if isNotFound(err) {
		return apperr.InvalidArgument("parent_id does not exist")
	}

	return err
}

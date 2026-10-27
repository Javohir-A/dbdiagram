package diagrams

import (
	"context"
	"errors"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
)

// requireDiagram is the single door to a diagram: every service method that
// takes a diagram guid goes through it, before anything is read or written.
// No access at all is NotFound (the diagram's existence is not revealed);
// some access but too little is PermissionDenied.
func requireDiagram(ctx context.Context, st storage.StorageI, actor, guid string, need model.Role) (model.Role, error) {
	role, err := st.Access().DiagramRole(ctx, actor, guid)
	if err != nil {
		return model.RoleNone, err
	}
	if role == model.RoleNone {
		return model.RoleNone, apperr.NotFound("diagram not found")
	}
	if !role.AtLeast(need) {
		return role, apperr.PermissionDenied("you need %s access to do this", need)
	}

	return role, nil
}

// requireFolder is requireDiagram for folders.
func requireFolder(ctx context.Context, st storage.StorageI, actor, guid string, need model.Role) (model.Role, error) {
	role, err := st.Access().FolderRole(ctx, actor, guid)
	if err != nil {
		return model.RoleNone, err
	}
	if role == model.RoleNone {
		return model.RoleNone, apperr.NotFound("folder not found")
	}
	if !role.AtLeast(need) {
		return role, apperr.PermissionDenied("you need %s access to do this", need)
	}

	return role, nil
}

// requireDestination checks the folder something is created in or moved to. The
// caller named a bad argument, not a missing resource, so a folder they cannot
// see is a 400 rather than a 404 about the thing they were operating on.
func requireDestination(ctx context.Context, st storage.StorageI, actor, folderGuid string) error {
	_, err := requireFolder(ctx, st, actor, folderGuid, model.RoleEditor)
	if isNotFound(err) {
		return apperr.InvalidArgument("folder_id does not exist")
	}

	return err
}

// presentDiagram shapes a diagram for one caller: their role, the public link
// only for owner-rank callers, and no folder they cannot see.
func presentDiagram(ctx context.Context, st storage.StorageI, actor string, d *model.Diagram, role model.Role) (*model.Diagram, error) {
	d.Role = role
	if role != model.RoleOwner {
		d.ShareToken = ""
	}
	if d.FolderID != "" {
		fr, err := st.Access().FolderRole(ctx, actor, d.FolderID)
		if err != nil {
			return nil, err
		}
		if fr == model.RoleNone {
			d.FolderID = ""
		}
	}

	return d, nil
}

// presentFolder is presentDiagram for folders: an unreachable parent is shown as top level.
func presentFolder(ctx context.Context, st storage.StorageI, actor string, f *model.Folder, role model.Role) (*model.Folder, error) {
	f.Role = role
	if f.ParentID != "" {
		pr, err := st.Access().FolderRole(ctx, actor, f.ParentID)
		if err != nil {
			return nil, err
		}
		if pr == model.RoleNone {
			f.ParentID = ""
		}
	}

	return f, nil
}

func isNotFound(err error) bool {
	var e *apperr.Error

	return errors.As(err, &e) && e.Kind == apperr.KindNotFound
}

package diagrams

import (
	"context"

	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
)

// ClaimUnowned gives every diagram and folder created before accounts existed
// to the user with this email. It backs the `claim` command, never an endpoint:
// it has no actor to authorize.
func (s *DiagramService) ClaimUnowned(ctx context.Context, email string) (diagrams, folders int64, err error) {
	u, err := s.deps.Users.ByEmail(ctx, email)
	if err != nil {
		return 0, 0, err
	}

	err = s.deps.Storage.Tx(ctx, func(st storage.StorageI) error {
		if folders, err = st.Folder().ClaimUnowned(ctx, u.Guid); err != nil {
			return err
		}

		diagrams, err = st.Diagram().ClaimUnowned(ctx, u.Guid)

		return err
	})

	return diagrams, folders, err
}

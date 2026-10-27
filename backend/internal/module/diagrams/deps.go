// Package diagrams owns the `diagrams` and `folders` tables: the DBML
// documents, their canvas layout, sharing and folders.
package diagrams

import (
	"context"
	"time"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

// Deps is what every service in this module is built from. There is no core
// module yet (no users, tenants or permissions); when auth arrives, its
// gateway interface is declared here, on the consumer side.
type Deps struct {
	Storage storage.StorageI
	Log     logger.Logger
	Cfg     *config.Config
	// Users is the only door into accounts. The interface is declared here, on
	// the consumer side, so core does not have to know who calls it.
	Users UserGateway
	// Now is the clock; nil means the real one. Only tests set it.
	Now func() time.Time
}

// UserGateway is the slice of the core module this one needs: turning a typed
// email (when sharing) or a stored id (an owner row) into a user.
type UserGateway interface {
	ByEmail(ctx context.Context, email string) (*model.User, error)
	ByID(ctx context.Context, guid string) (*model.User, error)
}

// now is the clock the history rules read; tests replace Deps.Now.
func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}

	return time.Now().UTC()
}

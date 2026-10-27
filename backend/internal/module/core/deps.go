// Package core owns accounts: registration, login, refresh-token sessions and
// the user's own profile. Every other module reaches a user only through the
// gateway interface it declares on its own side.
package core

import (
	"time"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/storage"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

type Deps struct {
	Storage storage.StorageI
	Log     logger.Logger
	Cfg     *config.Config
	// Now is the clock; nil means the real one. Only tests set it.
	Now func() time.Time
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}

	return time.Now().UTC()
}

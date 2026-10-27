package diagrams

import (
	"math"
	"strings"
	"unicode/utf8"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
)

// maxPositionKey bounds a table id; a DBML identifier is never close to this.
const maxPositionKey = 300

// cleanName trims a display name and enforces its bounds. Diagrams and folders
// share the rule, so the limits cannot drift apart.
func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)

	switch {
	case name == "":
		return "", apperr.InvalidArgument("name is required")
	case utf8.RuneCountInString(name) > config.MaxNameLength:
		return "", apperr.InvalidArgument("name must be at most %d characters", config.MaxNameLength)
	}

	return name, nil
}

// checkDBML rejects a document the table would never be asked to hold.
func checkDBML(dbml string) error {
	if len(dbml) > config.MaxDBMLSize {
		return apperr.PayloadTooLarge("dbml must be at most %d bytes", config.MaxDBMLSize)
	}

	return nil
}

// checkPositions keeps the layout column a plain id -> {x, y} object with sane
// numbers: the canvas trusts it blindly.
func checkPositions(positions map[string]model.Position) error {
	if len(positions) > config.MaxPositions {
		return apperr.InvalidArgument("positions must have at most %d entries", config.MaxPositions)
	}

	for id, p := range positions {
		if id == "" || len(id) > maxPositionKey {
			return apperr.InvalidArgument("positions has an invalid table id")
		}
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.Abs(p.X) > config.MaxCoordinate || math.Abs(p.Y) > config.MaxCoordinate {
			return apperr.InvalidArgument("positions[%q] is out of range", id)
		}
	}

	return nil
}

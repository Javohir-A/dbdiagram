// Package postgres holds the SQL. Business decisions do not live here.
package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/lib/pq"

	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

type rowScanner interface {
	Scan(dest ...any) error
}

// HandleDatabaseError turns a driver error into an apperr and logs it. Every
// repo method returns its error through this function, so a pq error never
// reaches the HTTP layer as a raw string.
func HandleDatabaseError(err error, log logger.Logger, message string) error {
	if err == nil {
		return nil
	}

	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return err
	}

	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound("this information does not exist")
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code.Name() {
		case "unique_violation":
			return apperr.AlreadyExists("already exists")
		case "foreign_key_violation":
			return apperr.FailedPrecondition("the elements related to this one must be removed first")
		case "check_violation", "not_null_violation", "invalid_text_representation":
			return apperr.InvalidArgument("invalid value for one of the fields")
		}
	}

	log.Error(message, logger.Error(err))

	return apperr.Internal(err, "%s", message)
}

// NotDeleted is the soft-delete guard. Every read carries it — rows are never
// removed, they get a deleted_at stamp.
func NotDeleted(alias string) squirrel.Eq { return squirrel.Eq{alias + ".deleted_at": nil} }

// PrepareLimit normalises paging so a missing page/limit cannot turn into a
// full table scan or a negative OFFSET.
func PrepareLimit(page, limit, defaultLimit, maxLimit int32) (offset, count uint64) {
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if page < 1 {
		page = 1
	}

	return uint64(page-1) * uint64(limit), uint64(limit)
}

// TimeString formats a timestamptz the way the API carries it.
func TimeString(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}

	return t.Time.UTC().Format(time.RFC3339)
}

// likeEscaper keeps user input from acting as LIKE wildcards.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Contains builds the ILIKE pattern for a plain-text search.
func Contains(search string) string { return "%" + likeEscaper.Replace(search) + "%" }

// Field is one entry of a list's filter/sort registry: the public name maps to
// a SQL expression, never the other way round.
type Field struct {
	Expr string
	Kind FieldKind
}

type FieldKind int

const (
	KindText FieldKind = iota
	KindTime
	KindGuid // nullable uuid: an empty value means "no value"
)

// ApplyFilters validates every filter against the registry and returns the
// WHERE conditions. Unknown fields, types or malformed values are rejected, so
// nothing the client sends is ever used as an identifier.
func ApplyFilters(filters []model.Filter, allowed map[string]Field) (squirrel.And, error) {
	where := squirrel.And{}

	for _, f := range filters {
		field, ok := allowed[f.Field]
		if !ok {
			return nil, apperr.InvalidArgument("unknown filter field %q", f.Field)
		}

		switch field.Kind {
		case KindText:
			switch f.Type {
			case "eq":
				where = append(where, squirrel.Expr(field.Expr+" = ?", f.Value))
			case "neq":
				where = append(where, squirrel.Expr(field.Expr+" <> ?", f.Value))
			case "search":
				where = append(where, squirrel.Expr(field.Expr+` ILIKE ? ESCAPE '\'`, Contains(f.Value)))
			default:
				return nil, apperr.InvalidArgument("filter type %q is not supported for %q", f.Type, f.Field)
			}
		case KindTime:
			ops := map[string]string{"eq": "=", "neq": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}
			op, ok := ops[f.Type]
			if !ok {
				return nil, apperr.InvalidArgument("filter type %q is not supported for %q", f.Type, f.Field)
			}
			ts, err := time.Parse(time.RFC3339, f.Value)
			if err != nil {
				return nil, apperr.InvalidArgument("filter value for %q must be an RFC3339 time", f.Field)
			}
			where = append(where, squirrel.Expr(field.Expr+" "+op+" ?", ts))
		case KindGuid:
			if f.Type != "eq" && f.Type != "neq" {
				return nil, apperr.InvalidArgument("filter type %q is not supported for %q", f.Type, f.Field)
			}
			if f.Value == "" {
				if f.Type == "eq" {
					where = append(where, squirrel.Expr(field.Expr+" IS NULL"))
				} else {
					where = append(where, squirrel.Expr(field.Expr+" IS NOT NULL"))
				}
				continue
			}
			if _, err := uuid.Parse(f.Value); err != nil {
				return nil, apperr.InvalidArgument("filter value for %q must be a uuid", f.Field)
			}
			if f.Type == "eq" {
				where = append(where, squirrel.Expr(field.Expr+" = ?", f.Value))
			} else {
				// NULL <> x is NULL, so unfiled rows must be kept explicitly.
				where = append(where, squirrel.Expr("("+field.Expr+" IS NULL OR "+field.Expr+" <> ?)", f.Value))
			}
		}
	}

	return where, nil
}

// ApplySort validates the requested order and appends the stable tiebreak: two
// rows with the same sort key must not swap places between pages.
func ApplySort(sort []model.SortBy, allowed map[string]string, defaultOrder, tiebreak string) (string, error) {
	if len(sort) == 0 {
		return defaultOrder + ", " + tiebreak, nil
	}

	parts := make([]string, 0, len(sort)+1)
	for _, s := range sort {
		expr, ok := allowed[s.Field]
		if !ok {
			return "", apperr.InvalidArgument("unknown sort field %q", s.Field)
		}
		if s.Type != "asc" && s.Type != "desc" {
			return "", apperr.InvalidArgument("sort type for %q must be asc or desc", s.Field)
		}
		parts = append(parts, fmt.Sprintf("%s %s", expr, s.Type))
	}

	return strings.Join(append(parts, tiebreak), ", "), nil
}

// encodePositions is the one way a layout reaches the jsonb column. A nil map
// is stored as {}: json.Marshal would write null, which the column's CHECK
// (an object) rightly refuses.
func encodePositions(positions map[string]model.Position) ([]byte, error) {
	if positions == nil {
		return []byte("{}"), nil
	}

	raw, err := json.Marshal(positions)
	if err != nil {
		return nil, apperr.Internal(err, "encode positions")
	}

	return raw, nil
}

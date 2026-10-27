package handler

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
)

var errInvalidQueryParam = errors.New("invalid query parameter")

// ParsePageQueryParam reads ?page=. A missing value is the first page.
func ParsePageQueryParam(c *gin.Context) (int32, error) {
	raw := c.Query("page")
	if raw == "" {
		return 1, nil
	}

	page, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || page < 1 {
		return 0, errInvalidQueryParam
	}

	return int32(page), nil
}

// ParseLimitQueryParam reads ?limit=. A missing value is the default page size;
// anything above MaxLimit is clamped rather than rejected, so a client asking
// for everything gets a bounded page instead of a table scan.
func ParseLimitQueryParam(c *gin.Context) (int32, error) {
	raw := c.Query("limit")
	if raw == "" {
		return config.DefaultLimit, nil
	}

	limit, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || limit < 1 {
		return 0, errInvalidQueryParam
	}

	if int32(limit) > config.MaxLimit {
		return config.MaxLimit, nil
	}

	return int32(limit), nil
}

// ParseFilters reads ?filters= as a JSON array. The repository validates field
// names against its registry before they reach SQL and rejects unknown ones.
func ParseFilters(c *gin.Context) ([]model.Filter, error) {
	raw := c.Query("filters")
	if raw == "" {
		return nil, nil
	}

	var filters []model.Filter
	if err := json.Unmarshal([]byte(raw), &filters); err != nil {
		return nil, errInvalidQueryParam
	}

	return filters, nil
}

// ParseSort reads ?sort= as a JSON array. List repositories reject unknown
// fields and directions before building ORDER BY.
func ParseSort(c *gin.Context) ([]model.SortBy, error) {
	raw := c.Query("sort")
	if raw == "" {
		return nil, nil
	}

	var sort []model.SortBy
	if err := json.Unmarshal([]byte(raw), &sort); err != nil {
		return nil, errInvalidQueryParam
	}

	return sort, nil
}

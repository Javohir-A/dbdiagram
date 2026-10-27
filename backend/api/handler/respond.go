package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"gitlab.com/udevs/dbdiagram/backend/api/dto"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/pkg/logger"
)

// respond writes the value or the error. It is the only place a handler turns
// a service result into HTTP, so status mapping lives in one function.
func (h *Handler) respond(c *gin.Context, value any, err error) {
	if err != nil {
		status, e := apperr.HTTPStatus(err)

		// 5xx is ours to fix and is logged with the cause; 4xx is the caller's
		// and would only fill the log with noise.
		if status >= http.StatusInternalServerError {
			h.log.Error("request failed",
				logger.String("path", c.FullPath()),
				logger.String("request_id", c.GetString("request_id")),
				logger.String("code", e.Code),
				logger.Error(e.Err))
		}

		c.AbortWithStatusJSON(status, dto.Error{Error: dto.ResponseError{Code: e.Code, Message: e.Message}})

		return
	}

	c.JSON(http.StatusOK, value)
}

// respondOK answers a handler that has nothing to return.
func (h *Handler) respondOK(c *gin.Context, err error) {
	h.respond(c, dto.Empty{}, err)
}

func (h *Handler) badRequest(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, dto.Error{
		Error: dto.ResponseError{Code: apperr.CodeBadRequest, Message: message},
	})
}

// bindJSON reads the body into dst. On failure it has already answered and
// returns false, so the handler just returns.
func (h *Handler) bindJSON(c *gin.Context, dst any) bool {
	err := c.ShouldBindJSON(dst)
	if err == nil {
		return true
	}

	var (
		tooLarge *http.MaxBytesError
		invalid  validator.ValidationErrors
	)

	switch {
	case errors.As(err, &tooLarge):
		h.respond(c, nil, apperr.PayloadTooLarge("request body must be at most %d bytes", tooLarge.Limit))
	case errors.As(err, &invalid):
		fields := make([]string, 0, len(invalid))
		for _, f := range invalid {
			fields = append(fields, strings.ToLower(f.Field()))
		}
		h.badRequest(c, fmt.Sprintf("%s is required", strings.Join(fields, ", ")))
	default:
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.Error{
			Error: dto.ResponseError{Code: apperr.CodeInvalidJSON, Message: "request body is not valid JSON of the expected shape"},
		})
	}

	return false
}

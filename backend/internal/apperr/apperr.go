// Package apperr holds the domain errors. The service layer does not know HTTP
// and the HTTP layer does not know SQL; both meet here.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Kind is the class of an error — what the HTTP layer maps to a status code.
type Kind int

const (
	KindInternal Kind = iota
	KindInvalidArgument
	KindNotFound
	KindAlreadyExists
	KindFailedPrecondition
	KindPayloadTooLarge
	KindUnavailable
	KindUnauthenticated
	KindPermissionDenied
	KindTooManyRequests
	KindRevisionConflict
)

// Stable machine codes. Clients switch on these, not on the message.
const (
	CodeBadRequest       = "BAD_REQUEST"
	CodeInvalidJSON      = "INVALID_JSON"
	CodeNotFound         = "NOT_FOUND"
	CodeAlreadyExists    = "ALREADY_EXISTS"
	CodeConflict         = "CONFLICT"
	CodePayloadTooLarge  = "PAYLOAD_TOO_LARGE"
	CodeUnavailable      = "UNAVAILABLE"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeRateLimited      = "RATE_LIMITED"
	CodeRevisionConflict = "REVISION_CONFLICT"
	CodeInternal         = "INTERNAL"
)

// Error carries a machine code and a message safe to hand to the caller. Err
// is the cause: it is logged and never sent.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}

	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func InvalidArgument(format string, a ...any) *Error {
	return &Error{Kind: KindInvalidArgument, Code: CodeBadRequest, Message: fmt.Sprintf(format, a...)}
}

func NotFound(format string, a ...any) *Error {
	return &Error{Kind: KindNotFound, Code: CodeNotFound, Message: fmt.Sprintf(format, a...)}
}

func AlreadyExists(format string, a ...any) *Error {
	return &Error{Kind: KindAlreadyExists, Code: CodeAlreadyExists, Message: fmt.Sprintf(format, a...)}
}

// FailedPrecondition is the "clean this up first" error.
func FailedPrecondition(format string, a ...any) *Error {
	return &Error{Kind: KindFailedPrecondition, Code: CodeConflict, Message: fmt.Sprintf(format, a...)}
}

func PayloadTooLarge(format string, a ...any) *Error {
	return &Error{Kind: KindPayloadTooLarge, Code: CodePayloadTooLarge, Message: fmt.Sprintf(format, a...)}
}

func Unauthenticated(format string, a ...any) *Error {
	return &Error{Kind: KindUnauthenticated, Code: CodeUnauthorized, Message: fmt.Sprintf(format, a...)}
}

func PermissionDenied(format string, a ...any) *Error {
	return &Error{Kind: KindPermissionDenied, Code: CodeForbidden, Message: fmt.Sprintf(format, a...)}
}

func TooManyRequests(format string, a ...any) *Error {
	return &Error{Kind: KindTooManyRequests, Code: CodeRateLimited, Message: fmt.Sprintf(format, a...)}
}

// RevisionConflict means somebody else changed the document since the caller loaded it.
func RevisionConflict(format string, a ...any) *Error {
	return &Error{Kind: KindRevisionConflict, Code: CodeRevisionConflict, Message: fmt.Sprintf(format, a...)}
}

// Unavailable wraps a dependency that is down; the cause stays in the log.
func Unavailable(err error, format string, a ...any) *Error {
	return &Error{
		Kind:    KindUnavailable,
		Code:    CodeUnavailable,
		Message: "Service Unavailable",
		Err:     fmt.Errorf(fmt.Sprintf(format, a...)+": %w", err),
	}
}

// Internal wraps a cause the caller must not see. The message sent out is
// always the same; the cause goes to the log.
func Internal(err error, format string, a ...any) *Error {
	return &Error{
		Kind:    KindInternal,
		Code:    CodeInternal,
		Message: "Internal Server Error",
		Err:     fmt.Errorf(fmt.Sprintf(format, a...)+": %w", err),
	}
}

var status = map[Kind]int{
	KindInternal:           http.StatusInternalServerError,
	KindInvalidArgument:    http.StatusBadRequest,
	KindNotFound:           http.StatusNotFound,
	KindAlreadyExists:      http.StatusConflict,
	KindFailedPrecondition: http.StatusConflict,
	KindPayloadTooLarge:    http.StatusRequestEntityTooLarge,
	KindUnavailable:        http.StatusServiceUnavailable,
	KindUnauthenticated:    http.StatusUnauthorized,
	KindPermissionDenied:   http.StatusForbidden,
	KindTooManyRequests:    http.StatusTooManyRequests,
	KindRevisionConflict:   http.StatusConflict,
}

// HTTPStatus resolves any error into a status code and a typed error. An error
// that is not an *apperr.Error is treated as internal — never leaked as-is.
func HTTPStatus(err error) (int, *Error) {
	var e *Error
	if !errors.As(err, &e) {
		e = Internal(err, "unhandled error")
	}

	code, ok := status[e.Kind]
	if !ok {
		code = http.StatusInternalServerError
	}

	return code, e
}

// Package httperr defines typed HTTP errors and maps them to HTTP statuses
// exactly as Laravel's centralized handler does (see docs/api-inventory.md).
// Business layers (service/repository) return these types; only the HTTP layer
// converts them to a status + body.
package httperr

import "fmt"

type Kind int

const (
	KindBadRequest Kind = iota
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindValidation
	KindTooManyRequests
	KindInternal
)

// Error carries a category and a client-safe message. Eg it never contains
// SQL/driver details; those live in Err for server-side logging only.
type Error struct {
	Kind    Kind
	Message string
	Errors  map[string][]string // for KindValidation, mirrors Laravel's errors map
	Err     error               // optional underlying error, logged, never sent to client
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

func NotFound(msg string) *Error { return &Error{Kind: KindNotFound, Message: msg} }
func Internal(err error) *Error {
	return &Error{Kind: KindInternal, Message: "Server error.", Err: err}
}
func BadRequest(msg string) *Error   { return &Error{Kind: KindBadRequest, Message: msg} }
func Unauthorized(msg string) *Error { return &Error{Kind: KindUnauthorized, Message: msg} }
func Forbidden(msg string) *Error    { return &Error{Kind: KindForbidden, Message: msg} }

// Validation builds the exact 422 shape Laravel produces:
//
//	{"message":"The given data was invalid.","errors":{field:[messages]}}
func Validation(errors map[string][]string) *Error {
	return &Error{Kind: KindValidation, Message: "The given data was invalid.", Errors: errors}
}

// Unprocessable is a plain 422 with a custom message (Laravel controllers that
// return response()->json([...], 422) directly).
func Unprocessable(msg string) *Error { return &Error{Kind: KindValidation, Message: msg} }

func TooManyRequests(msg string) *Error { return &Error{Kind: KindTooManyRequests, Message: msg} }

// As extracts an *Error from err, or nil when err is nil or of another type.
func As(err error) *Error {
	if err == nil {
		return nil
	}
	e, ok := err.(*Error)
	if ok {
		return e
	}
	return nil
}

// Status maps an error to the HTTP status it represents.
func Status(err error) int {
	if e := As(err); e != nil {
		switch e.Kind {
		case KindNotFound:
			return 404
		case KindBadRequest:
			return 400
		case KindUnauthorized:
			return 401
		case KindForbidden:
			return 403
		case KindConflict:
			return 409
		case KindValidation:
			return 422
		case KindTooManyRequests:
			return 429
		}
	}
	return 500
}

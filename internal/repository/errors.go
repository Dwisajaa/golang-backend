package repository

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// ErrNotFound is the sentinel a repository returns when no row exists.
// Services map it to a client-safe 404.
var ErrNotFound = errors.New("repository: not found")

// ErrDuplicateEmail is returned by the user repository when an INSERT hits the
// unique email constraint (MySQL error 1062), translated away from the driver.
var ErrDuplicateEmail = errors.New("repository: duplicate email")

// ErrDuplicate is a generic unique-constraint violation translated from the
// driver (used when the colliding key is not a classified one).
var ErrDuplicate = errors.New("repository: duplicate row")

// ErrDuplicateTechnicianCode is the unique violation on technician_code.
var ErrDuplicateTechnicianCode = errors.New("repository: duplicate technician code")

// ErrDuplicateName / ErrDuplicateSlug classify service_categories unique
// violations so the service can produce field-specific 422 messages.
var (
	ErrDuplicateName = errors.New("repository: duplicate name")
	ErrDuplicateSlug = errors.New("repository: duplicate slug")
)

// classifyServiceCategoryDuplicate maps a 1062 key to the matching sentinel
// (or a generic ErrDuplicate when the key is unknown).
// classifyServiceDuplicate maps a 1062 key to the matching sentinel for the
// services table (or a generic ErrDuplicate).
func classifyServiceDuplicate(err error) error {
	key, ok := duplicateTarget(err)
	if !ok {
		return err
	}
	switch {
	case key == "services_name_unique":
		return ErrDuplicateName
	case key == "services_slug_unique":
		return ErrDuplicateSlug
	default:
		return ErrDuplicate
	}
}

func classifyServiceCategoryDuplicate(err error) error {
	key, ok := duplicateTarget(err)
	if !ok {
		return err
	}
	switch {
	case key == "service_categories_name_unique":
		return ErrDuplicateName
	case key == "service_categories_slug_unique":
		return ErrDuplicateSlug
	default:
		return ErrDuplicate
	}
}

// duplicateTarget reports the colliding unique key name for a MySQL 1062 error
// (e.g. "technician_code" / "users.email"). Returns false for other errors.
func duplicateTarget(err error) (string, bool) {
	var myErr *mysql.MySQLError
	if !errors.As(err, &myErr) || myErr.Number != 1062 {
		return "", false
	}
	const marker = "for key '"
	idx := strings.Index(myErr.Message, marker)
	if idx < 0 {
		return "", true // 1062 but unparsed message
	}
	key := myErr.Message[idx+len(marker):]
	if end := strings.IndexByte(key, '\''); end >= 0 {
		key = key[:end]
	}
	return key, true
}

// isDuplicate reports any MySQL 1062 violation without classifying the key.
func isDuplicate(err error) bool {
	_, ok := duplicateTarget(err)
	return ok
}

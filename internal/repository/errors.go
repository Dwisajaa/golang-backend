package repository

import "errors"

// ErrNotFound is the sentinel a repository returns when no row exists.
// Services map it to a client-safe 404.
var ErrNotFound = errors.New("repository: not found")

// ErrDuplicateEmail is returned by the user repository when an INSERT hits the
// unique email constraint (MySQL error 1062), translated away from the driver.
var ErrDuplicateEmail = errors.New("repository: duplicate email")

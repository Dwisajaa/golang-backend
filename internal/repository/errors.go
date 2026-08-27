package repository

import "errors"

// ErrNotFound is the sentinel a repository returns when no row exists.
// Services map it to a client-safe 404.
var ErrNotFound = errors.New("repository: not found")

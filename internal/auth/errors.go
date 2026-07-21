package auth

import "errors"

// ErrNoSession means the request carried no usable session. Handlers translate
// this to 401; it is not an internal error and must not be logged as one.
var ErrNoSession = errors.New("no active session")

// ErrNotFound means the requested record does not exist.
var ErrNotFound = errors.New("not found")

package context

import "errors"

// ErrAlreadyCommitted is returned when trying to add actions or commit
// after the RequestContext has already been committed.
var ErrAlreadyCommitted = errors.New("request context already committed")

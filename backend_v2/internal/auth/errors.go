package auth

import "aladin/backend_v2/internal/apperror"

var (
	ErrNotFound = apperror.ErrNotFound
	ErrConflict = apperror.ErrConflict
)

type BadRequest = apperror.BadRequest

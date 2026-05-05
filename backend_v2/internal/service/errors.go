package service

import "errors"

var ErrNotFound = errors.New("not found")

var ErrConflict = errors.New("conflict")

type BadRequest string

func (e BadRequest) Error() string {
	return string(e)
}

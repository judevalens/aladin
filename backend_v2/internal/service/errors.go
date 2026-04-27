package service

import "errors"

var ErrNotFound = errors.New("not found")

type BadRequest string

func (e BadRequest) Error() string {
	return string(e)
}

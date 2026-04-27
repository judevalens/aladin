package service

import (
	"strings"

	"github.com/google/uuid"
)

func newID(prefix string) string {
	return prefix + uuid.NewString()
}

func NewID(prefix string, suffix string) string {
	return prefix + uuid.NewString() + suffix
}

func TrimStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

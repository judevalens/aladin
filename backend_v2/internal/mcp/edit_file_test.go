package mcpserver

import (
	"errors"
	"testing"

	"aladin/backend_v2/internal/docsurface"
)

func TestApplyStringEdit(t *testing.T) {
	const src = "alpha\nbeta\nalpha\n"

	t.Run("single match replaces once", func(t *testing.T) {
		out, n, err := docsurface.ApplyStringEdit("hello world", "world", "there", false)
		if err != nil || n != 1 || out != "hello there" {
			t.Fatalf("got (%q, %d, %v)", out, n, err)
		}
	})

	t.Run("not found errors", func(t *testing.T) {
		_, _, err := docsurface.ApplyStringEdit(src, "gamma", "x", false)
		if !errors.Is(err, docsurface.ErrEditNotFound) {
			t.Fatalf("want errEditNotFound, got %v", err)
		}
	})

	t.Run("ambiguous without replace_all errors with count", func(t *testing.T) {
		_, n, err := docsurface.ApplyStringEdit(src, "alpha", "x", false)
		if !errors.Is(err, docsurface.ErrEditAmbiguous) || n != 2 {
			t.Fatalf("want errEditAmbiguous count=2, got (%d, %v)", n, err)
		}
	})

	t.Run("replace_all replaces every occurrence", func(t *testing.T) {
		out, n, err := docsurface.ApplyStringEdit(src, "alpha", "ALPHA", true)
		if err != nil || n != 2 || out != "ALPHA\nbeta\nALPHA\n" {
			t.Fatalf("got (%q, %d, %v)", out, n, err)
		}
	})

	t.Run("unique-by-context single replace", func(t *testing.T) {
		out, n, err := docsurface.ApplyStringEdit(src, "beta\nalpha", "beta\nGAMMA", false)
		if err != nil || n != 1 || out != "alpha\nbeta\nGAMMA\n" {
			t.Fatalf("got (%q, %d, %v)", out, n, err)
		}
	})
}

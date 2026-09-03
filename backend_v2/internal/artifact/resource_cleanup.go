package artifact

import (
	"errors"
	"fmt"
)

func cleanupStoredResource(store ArtifactFileStore, storageKey string, operationErr error) error {
	if cleanupErr := store.DeleteResource(storageKey); cleanupErr != nil {
		return errors.Join(operationErr, fmt.Errorf("compensate stored resource %q: %w", storageKey, cleanupErr))
	}
	return operationErr
}

package idgen

import (
	"strings"

	"github.com/google/uuid"
)

// UUID generate UUID
func UUID() (string, error) {
	id, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(id.String(), "-", ""), nil
}

// GenerateGlobalID generates global unique id
func GenerateGlobalID() (globalID string, err error) {
	return UUID()
}

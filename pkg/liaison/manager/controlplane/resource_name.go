package controlplane

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

const resourceNameRandomBytes = 4

// normalizeResourceName keeps an explicit name or generates a compact default.
func normalizeResourceName(name, prefix string) (string, error) {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed, nil
	}

	random := make([]byte, resourceNameRandomBytes)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate %s name: %w", strings.ToLower(prefix), err)
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(random)), nil
}

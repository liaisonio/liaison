package aigateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"
)

// NewKey returns the once-visible secret and its persistence digest. Never store
// or log the secret. Lifecycle/owner authorization is a control-plane concern.
func NewKey() (secret, digest string, err error) {
	var entropy [32]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		return "", "", err
	}
	secret = "lia_ai_" + base64.RawURLEncoding.EncodeToString(entropy[:])
	return secret, KeyDigest(secret), nil
}

func KeyDigest(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func validModel(model string) bool {
	return model != "" && len(model) <= 256 && strings.IndexFunc(model, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	}) == -1
}

// AllowedModels intersects explicit aliases in a key scope and access mappings.
// Empty scopes deny all; there is no implicit wildcard or upstream-name bypass.
// Returned maps are copies and can safely be used as a per-request snapshot.
func AllowedModels(scope []string, mappings map[string]string) map[string]string {
	result := make(map[string]string)
	for _, alias := range scope {
		upstream, ok := mappings[alias]
		if ok && validModel(alias) && validModel(upstream) {
			result[alias] = upstream
		}
	}
	return result
}

func ModelAliases(allowed map[string]string) []string {
	aliases := make([]string, 0, len(allowed))
	for alias := range allowed {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

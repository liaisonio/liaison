package codex

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSkillsOnlyExposeEnabledProjectCatalog(t *testing.T) {
	items, local, err := parseSkills([]byte(`{"data":[{"cwd":"/project","errors":[],"skills":[{"name":"review","description":"Read code","path":"/skills/review/SKILL.md","enabled":true},{"name":"disabled","path":"/skills/off/SKILL.md","enabled":false},{"name":"relative","path":"../secret","enabled":true}]},{"cwd":"/foreign","skills":[{"name":"foreign","path":"/private/SKILL.md","enabled":true}]}]}`), "/project")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].ID, 32)
	require.Equal(t, "review", items[0].Name)
	require.Equal(t, "/skills/review/SKILL.md", local[items[0].ID].Path)
	_, _, err = parseSkills([]byte(`{"data":[{"cwd":"/project","errors":[{"message":"secret"}]}]}`), "/project")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret")
	_, _, err = parseSkills([]byte(`{}`), "/project")
	require.Error(t, err)
}

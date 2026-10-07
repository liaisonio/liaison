package proto

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWebIDERequestValidation(t *testing.T) {
	base := WebIDERequest{Version: 1, OwnerID: "123", Action: "start", AccessID: "access", InstallationID: "installation", Project: "/work"}
	require.True(t, base.Valid())
	empty := base
	empty.Project = ""
	require.True(t, empty.Valid(), "IDE can start without a folder")
	for _, change := range []func(*WebIDERequest){func(q *WebIDERequest) { q.OwnerID = "" }, func(q *WebIDERequest) { q.AccessID = "../a" }, func(q *WebIDERequest) { q.InstanceID = "unexpected" }, func(q *WebIDERequest) { q.Version = 2 }, func(q *WebIDERequest) { q.Action = "exec" }} {
		q := base
		change(&q)
		require.False(t, q.Valid())
	}
}

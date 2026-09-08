package accesssession

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPublicReferenceOwnerScopeAndNoToken(t *testing.T) {
	r := NewRegistry()
	_, remove, err := r.Register(Handle{Descriptor: Descriptor{ID: "secret-token", UserID: 7, AccessID: 3, Protocol: ProtocolMySQL}, Data: stubData{}})
	require.NoError(t, err)
	t.Cleanup(remove)
	ref := PublicReference("secret-token")
	require.Regexp(t, `^conn_[a-f0-9]{24}$`, ref)
	d, err := r.DescribeReference(context.Background(), ref, 7)
	require.NoError(t, err)
	require.Empty(t, d.ID)
	require.Equal(t, uint(3), d.AccessID)
	for _, user := range []uint{0, 8} {
		_, err = r.DescribeReference(context.Background(), ref, user)
		require.ErrorIs(t, err, ErrHandleNotFound)
	}
	remove()
	_, err = r.DescribeReference(context.Background(), ref, 7)
	require.ErrorIs(t, err, ErrHandleNotFound)
}

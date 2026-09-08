package controlplane

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAuditBusinessMethodsFailClosedWithoutFeatureGrant(t *testing.T) {
	cp, r := newTestControlPlane(t)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	cp.authorizeFeature = nil
	_, err := cp.ListManagementAudits(context.Background(), nil)
	require.ErrorIs(t, err, iam.ErrForbidden)
	_, err = cp.ListWebDataAuditEntries(context.Background(), nil)
	require.ErrorIs(t, err, iam.ErrForbidden)
	cp.authorizeFeature = func(_ context.Context, code string) error {
		require.Equal(t, iam.FeatureAudit, code)
		return iam.ErrForbidden
	}
	_, err = cp.ListWebDataAuditEntries(context.Background(), nil)
	require.ErrorIs(t, err, iam.ErrForbidden)
}

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/management"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

// Real SQLite DAO + controlplane resource filters + Casbin, not mocked tenancy.
func TestManagementToolsThreeUserResourceIsolation(t *testing.T) {
	cp, r := newTestControlPlane(t)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	admin, a, b := seedResourceScopeUsers(t, r)
	root, err := r.GetRootOrganization()
	require.NoError(t, err)
	for index, ctx := range []context.Context{a, b} {
		name := fmt.Sprintf("owner-%d", index)
		_, err := cp.CreateEdge(ctx, &v1.CreateEdgeRequest{Name: name})
		require.NoError(t, err)
		device := &model.Device{Name: name, Fingerprint: name}
		require.NoError(t, r.CreateDevice(device))
		require.NoError(t, claimResource(ctx, r, resourceDevice, uint64(device.ID)))
		app := &model.Application{Name: name, DeviceID: device.ID, EdgeIDs: model.UintSlice{}, IP: "127.0.0.1", Port: 8080, ApplicationType: model.ApplicationType("http")}
		require.NoError(t, r.CreateApplication(app))
		require.NoError(t, claimResource(ctx, r, resourceApplication, uint64(app.ID)))
	}
	iamService, err := iam.NewIAMService(r)
	require.NoError(t, err)
	adminUser, err := r.GetUserByID(admin.Value("user_id").(uint))
	require.NoError(t, err)
	require.NoError(t, iamService.SetUserFeaturePolicy(adminUser, []string{iam.FeatureHomeAI}))
	source, err := management.NewSource(cp, iamService)
	require.NoError(t, err)
	registrations, err := source.Snapshot(context.Background())
	require.NoError(t, err)
	for _, registration := range registrations {
		if registration.Descriptor.ID.Name != "list" {
			continue
		}
		t.Run(registration.Descriptor.ID.Namespace, func(t *testing.T) {
			var firstID string
			for index, ctx := range []context.Context{a, b, admin} {
				userID, _ := actorUserID(ctx)
				executor, err := registration.Factory.Bind(context.Background(), tool.ToolBinding{SessionKind: tool.SessionManagement, Principal: tool.Principal{UserID: userID, OrganizationID: root.ID}})
				require.NoError(t, err)
				output, err := executor.Execute(context.Background(), json.RawMessage(`{}`))
				require.NoError(t, err)
				var result struct {
					Total int `json:"total"`
					Items []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"items"`
				}
				require.NoError(t, json.Unmarshal(output.Content, &result))
				if index == 2 {
					require.Equal(t, 2, result.Total)
					require.Len(t, result.Items, 2)
				} else {
					require.Equal(t, 1, result.Total)
					require.Len(t, result.Items, 1)
					require.Equal(t, fmt.Sprintf("owner-%d", index), result.Items[0].Name)
					if index == 0 {
						firstID = result.Items[0].ID
					}
				}
			}
			for _, get := range registrations {
				if get.Descriptor.ID.Namespace != registration.Descriptor.ID.Namespace || get.Descriptor.ID.Name != "get" {
					continue
				}
				userID, _ := actorUserID(b)
				executor, err := get.Factory.Bind(context.Background(), tool.ToolBinding{SessionKind: tool.SessionManagement, Principal: tool.Principal{UserID: userID, OrganizationID: root.ID}})
				require.NoError(t, err)
				_, err = executor.Execute(context.Background(), json.RawMessage(fmt.Sprintf(`{"id":%q}`, firstID)))
				require.Error(t, err)
				var httpErr *HTTPError
				require.ErrorAs(t, err, &httpErr)
				require.Equal(t, 404, httpErr.Status())
			}
		})
	}
}

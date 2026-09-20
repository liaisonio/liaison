package lifecycle

import (
	"context"
	"encoding/json"
	"github.com/liaisonio/liaison/pkg/edge/frontierbound"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio"
	"github.com/stretchr/testify/require"
	"testing"
)

type lifecycleHandlers struct {
	frontierbound.FrontierBound
	handlers map[string]func(context.Context, geminio.Request, geminio.Response)
}

func (f *lifecycleHandlers) RegisterRPCHandler(name string, h func(context.Context, geminio.Request, geminio.Response)) error {
	f.handlers[name] = h
	return nil
}

type lifecycleRequest struct {
	geminio.Request
	data []byte
}

func (r lifecycleRequest) Data() []byte { return r.data }

type lifecycleResponse struct {
	geminio.Response
	data []byte
	err  error
}

func (r *lifecycleResponse) SetError(err error)  { r.err = err }
func (r *lifecycleResponse) SetData(data []byte) { r.data = data }

func TestUninstallRuntimeBindingAndDefaultOff(t *testing.T) {
	ctx := context.Background()
	bindings := []string{}
	for _, enabled := range []bool{false, true} {
		f := &lifecycleHandlers{handlers: map[string]func(context.Context, geminio.Request, geminio.Response){}}
		require.NoError(t, RegisterStatus(f, Identity{OS: "unsupported-test-platform"}, enabled, nil, false))
		statusResponse := &lifecycleResponse{}
		f.handlers[proto.RPCInstallationStatus](ctx, lifecycleRequest{}, statusResponse)
		require.NoError(t, statusResponse.err)
		var status proto.InstallationStatus
		require.NoError(t, json.Unmarshal(statusResponse.data, &status))
		require.Regexp(t, instancePattern, status.RuntimeID)
		require.False(t, status.CanUninstall)
		bindings = append(bindings, status.RuntimeID)
		cmd := validCommand() // A different process binding must never reach Prepare.
		data, err := json.Marshal(cmd)
		require.NoError(t, err)
		response := &lifecycleResponse{}
		f.handlers[proto.RPCUninstall](ctx, lifecycleRequest{data: data}, response)
		require.ErrorIs(t, response.err, ErrUnsupported)
	}
	require.NotEqual(t, bindings[0], bindings[1], "restart must rotate the runtime binding")
}

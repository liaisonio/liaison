package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/liaisonio/liaison/pkg/edge/frontierbound"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio"
)

func Status(ctx context.Context, id Identity, run Command) proto.InstallationStatus {
	result := proto.InstallationStatus{Version: 1, Platform: id.OS, Reason: "installation_unverified"}
	plan, err := Check(ctx, id, run)
	if err != nil {
		return result
	}
	result.InstanceID = plan.InstanceID
	result.Service = plan.Service
	result.InstallationKind = plan.Kind
	result.Legacy = plan.Legacy
	result.OwnershipVerified = true
	// Never advertise execution based only on a successful read-only probe.
	result.Reason = "uninstall_executor_unavailable"
	return result
}

// RegisterStatus adds a read-only handler without any unsolicited RPC to older
// Managers. Serializing probes bounds native child processes under repeated calls.
func RegisterStatus(fb frontierbound.FrontierBound, id Identity, allowed bool, hosts []string, insecureTLS bool) error {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	runtimeID := hex.EncodeToString(nonce[:])
	var mu sync.Mutex
	consumed := false
	if err := fb.RegisterRPCHandler(proto.RPCInstallationStatus, func(ctx context.Context, req geminio.Request, rsp geminio.Response) {
		mu.Lock()
		defer mu.Unlock()
		status := Status(ctx, id, NativeCommand)
		status.RuntimeID = runtimeID
		if allowed && !consumed && status.OwnershipVerified {
			status.CanUninstall = true
			status.Reason = ""
		}
		data, err := json.Marshal(status)
		if err != nil {
			rsp.SetError(err)
			return
		}
		rsp.SetData(data)
	}); err != nil {
		return err
	}
	return fb.RegisterRPCHandler(proto.RPCUninstall, func(ctx context.Context, req geminio.Request, rsp geminio.Response) {
		mu.Lock()
		defer mu.Unlock()
		if !allowed || consumed || len(req.Data()) > 4096 {
			rsp.SetError(ErrUnsupported)
			return
		}
		var command proto.UninstallCommand
		if err := json.Unmarshal(req.Data(), &command); err != nil {
			rsp.SetError(ErrUnsupported)
			return
		}
		if command.RuntimeID != runtimeID {
			rsp.SetError(ErrUnsupported)
			return
		}
		// A process accepts at most one destructive request. Reconnects do not
		// automatically execute pending tasks; expiry bounds delayed delivery.
		consumed = true
		if err := Prepare(ctx, id, command, hosts, insecureTLS, NativeCommand); err != nil {
			rsp.SetError(ErrUnsupported)
			return
		}
		rsp.SetData([]byte(`{"accepted":true}`))
	})
}

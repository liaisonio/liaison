package frontierbound

import (
	"errors"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio/application"
	"testing"
)

func TestMissingProbeRPC(t *testing.T) {
	for _, err := range []error{application.ErrRemoteRPCUnregistered, errors.New("remote rpc unregistered"), errors.New("no such rpc: " + proto.RPCTCPProbe)} {
		if !missingProbeRPC(err) {
			t.Errorf("missing capability not recognized: %v", err)
		}
	}
	for _, err := range []error{nil, errors.New("network timeout"), errors.New("no such rpc: unrelated"), errors.New("secret remote rpc unregistered detail")} {
		if missingProbeRPC(err) {
			t.Errorf("transport failure misclassified: %v", err)
		}
	}
}

package executor

import (
	"context"
	"errors"
	"fmt"

	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
)

type SessionDescriber interface {
	Describe(ctx context.Context, id string, userID uint) (accesssession.Descriptor, error)
}

// AttachmentBinder converts an opaque live handle into server-authored tool
// binding facts. Browser and model input can select a handle, but cannot set
// its access, application, protocol, capability or generation fields.
type AttachmentBinder struct {
	sessions SessionDescriber
}

func NewAttachmentBinder(sessions SessionDescriber) (*AttachmentBinder, error) {
	if sessions == nil {
		return nil, errors.New("access session describer is required")
	}
	return &AttachmentBinder{sessions: sessions}, nil
}

func (binder *AttachmentBinder) Bind(ctx context.Context, principal tool.Principal, handleID string) (tool.AttachmentSnapshot, error) {
	if principal.UserID == 0 {
		return tool.AttachmentSnapshot{}, errors.New("attachment principal is required")
	}
	descriptor, err := binder.sessions.Describe(ctx, handleID, principal.UserID)
	if err != nil {
		return tool.AttachmentSnapshot{}, fmt.Errorf("describe access session: %w", err)
	}
	capabilities := make([]tool.Capability, 0, len(descriptor.Capabilities))
	for _, capability := range descriptor.Capabilities {
		capabilities = append(capabilities, tool.Capability(capability))
	}
	return tool.AttachmentSnapshot{
		ID:             descriptor.ID,
		AccessID:       descriptor.AccessID,
		ApplicationID:  descriptor.ApplicationID,
		Protocol:       tool.Protocol(descriptor.Protocol),
		Capabilities:   capabilities,
		Generation:     descriptor.Generation,
		AccessHandleID: descriptor.ID,
	}, nil
}

package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Engine struct {
	lifecycleMu sync.Mutex
	catalog     *Catalog
	registry    *Registry
	disclosure  *DisclosureEngine
	policy      PolicyEvaluator
	middleware  []ToolMiddleware
}

func NewEngine(visibility VisibilityFilter, policy PolicyEvaluator, middleware ...ToolMiddleware) *Engine {
	catalog := NewCatalog()
	if policy == nil {
		policy = AllowAllPolicy{}
	}
	return &Engine{
		catalog:    catalog,
		registry:   NewRegistry(),
		disclosure: NewDisclosureEngine(catalog, visibility),
		policy:     policy,
		middleware: append([]ToolMiddleware(nil), middleware...),
	}
}

func (engine *Engine) Register(_ context.Context, registration ToolRegistration) error {
	return engine.register(registration, false)
}

func (engine *Engine) Replace(_ context.Context, registration ToolRegistration) error {
	return engine.register(registration, true)
}

func (engine *Engine) register(registration ToolRegistration, replace bool) error {
	if registration.Factory == nil {
		return fmt.Errorf("%w: factory is required", ErrInvalidDescriptor)
	}
	if err := ValidateDescriptor(registration.Descriptor); err != nil {
		return err
	}
	engine.lifecycleMu.Lock()
	defer engine.lifecycleMu.Unlock()
	existing := engine.catalog.byModelName(registration.Descriptor.ID.Namespace, registration.Descriptor.ID.Name)
	if !replace && len(existing) > 0 {
		return fmt.Errorf("%w: model-visible name %s", ErrAlreadyRegistered, registration.Descriptor.ID.ModelName())
	}
	if replace {
		for _, entry := range existing {
			if entry.descriptor.Source.ID != registration.Descriptor.Source.ID {
				return fmt.Errorf("%w: source %q cannot replace %s owned by %q", ErrToolUnavailable, registration.Descriptor.Source.ID, entry.descriptor.ID.String(), entry.descriptor.Source.ID)
			}
			if entry.descriptor.ID == registration.Descriptor.ID {
				continue
			}
			if _, err := engine.registry.setStatus(entry.descriptor.ID, ToolStatusDraining); err != nil && !errors.Is(err, ErrToolNotFound) {
				return fmt.Errorf("drain replaced tool %s: %w", entry.descriptor.ID.String(), err)
			}
			engine.catalog.setStatus(entry.descriptor.ID, ToolStatusDraining)
		}
	}
	generation, err := engine.registry.register(registration, replace)
	if err != nil {
		return err
	}
	engine.catalog.upsert(registration.Descriptor, generation, ToolStatusActive)
	return nil
}

func (engine *Engine) Enable(_ context.Context, id ToolID) error {
	engine.lifecycleMu.Lock()
	defer engine.lifecycleMu.Unlock()
	if _, err := engine.registry.setStatus(id, ToolStatusActive); err != nil {
		return err
	}
	if _, ok := engine.catalog.setStatus(id, ToolStatusActive); !ok {
		return fmt.Errorf("enable catalog entry %s: %w", id.String(), ErrToolNotFound)
	}
	return nil
}

func (engine *Engine) Disable(_ context.Context, id ToolID) error {
	engine.lifecycleMu.Lock()
	defer engine.lifecycleMu.Unlock()
	if _, err := engine.registry.setStatus(id, ToolStatusDisabled); err != nil {
		return err
	}
	if _, ok := engine.catalog.setStatus(id, ToolStatusDisabled); !ok {
		return fmt.Errorf("disable catalog entry %s: %w", id.String(), ErrToolNotFound)
	}
	return nil
}

func (engine *Engine) Drain(_ context.Context, id ToolID) error {
	engine.lifecycleMu.Lock()
	defer engine.lifecycleMu.Unlock()
	if _, err := engine.registry.setStatus(id, ToolStatusDraining); err != nil {
		return err
	}
	if _, ok := engine.catalog.setStatus(id, ToolStatusDraining); !ok {
		return fmt.Errorf("drain catalog entry %s: %w", id.String(), ErrToolNotFound)
	}
	return nil
}

// Unload stops new calls immediately. When force is false, an executing call is
// allowed to finish. When force is true, its lease context is cancelled.
func (engine *Engine) Unload(_ context.Context, id ToolID, force bool) (bool, error) {
	engine.lifecycleMu.Lock()
	defer engine.lifecycleMu.Unlock()
	_, complete, err := engine.registry.unload(id, force)
	if err != nil {
		return false, err
	}
	if complete {
		engine.catalog.remove(id)
	} else {
		engine.catalog.setStatus(id, ToolStatusDraining)
	}
	return complete, nil
}

func (engine *Engine) Status(id ToolID) (ToolStatus, uint64, int, bool) {
	return engine.registry.status(id)
}

func (engine *Engine) BuildSnapshot(ctx context.Context, request DisclosureRequest) (ToolSetSnapshot, error) {
	return engine.disclosure.BuildSnapshot(ctx, request)
}

func (engine *Engine) Search(ctx context.Context, request DisclosureRequest, query string) ([]ToolSummary, error) {
	return engine.disclosure.Search(ctx, request, query)
}

func (engine *Engine) Describe(ctx context.Context, request DisclosureRequest, ids []ToolID) ([]ToolDescriptor, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	request.Promoted = append(append([]ToolID(nil), request.Promoted...), ids...)
	request.Budget.MaxSchemas = max(request.Budget.MaxSchemas, len(ids)+DefaultDisclosureBudget().MaxAlwaysVisible)
	snapshot, err := engine.disclosure.BuildSnapshot(ctx, request)
	if err != nil {
		return nil, err
	}
	wanted := toolIDSet(ids)
	descriptors := make([]ToolDescriptor, 0, len(ids))
	for _, exposed := range snapshot.Tools {
		if _, ok := wanted[exposed.Descriptor.ID]; ok {
			descriptors = append(descriptors, cloneDescriptor(exposed.Descriptor))
		}
	}
	return descriptors, nil
}

func (engine *Engine) Execute(ctx context.Context, snapshot ToolSetSnapshot, invocation ToolInvocation) (ToolResult, error) {
	return engine.execute(ctx, snapshot, invocation, nil)
}

// ExecuteApproved re-evaluates current policy and only bypasses an approval
// decision when the single-use grant still matches the exact frozen call.
func (engine *Engine) ExecuteApproved(ctx context.Context, snapshot ToolSetSnapshot, invocation ToolInvocation, grant ApprovalGrant) (ToolResult, error) {
	return engine.execute(ctx, snapshot, invocation, &grant)
}

func (engine *Engine) execute(ctx context.Context, snapshot ToolSetSnapshot, invocation ToolInvocation, grant *ApprovalGrant) (ToolResult, error) {
	if invocation.ID == "" {
		return ToolResult{}, errors.New("tool invocation ID is required")
	}
	if invocation.Binding.ToolSnapshotID != snapshot.ID {
		return ToolResult{}, fmt.Errorf("%w: binding references snapshot %q, got %q", ErrStaleSnapshot, invocation.Binding.ToolSnapshotID, snapshot.ID)
	}
	exposed, ok := snapshot.Find(invocation.Call.ID)
	if !ok {
		return ToolResult{}, fmt.Errorf("%w: %s", ErrToolNotExposed, invocation.Call.ID.String())
	}
	if !descriptorMatchesSession(exposed.Descriptor, invocation.Binding.SessionKind) {
		return ToolResult{}, fmt.Errorf("%w: tool is not available for this session kind", ErrPolicyDenied)
	}
	if invocation.RegistrationGeneration != 0 && invocation.RegistrationGeneration != exposed.RegistrationGeneration {
		return ToolResult{}, fmt.Errorf("%w: invocation generation does not match exposure", ErrStaleSnapshot)
	}
	invocation.RegistrationGeneration = exposed.RegistrationGeneration
	if err := validateAttachmentGeneration(snapshot, invocation.Binding.Attachment); err != nil {
		return ToolResult{}, err
	}

	decision, err := engine.policy.Evaluate(ctx, invocation, exposed.Descriptor)
	if err != nil {
		return ToolResult{}, fmt.Errorf("evaluate policy for %s: %w", invocation.Call.ID.String(), err)
	}
	if len(decision.NormalizedInput) != 0 {
		if !json.Valid(decision.NormalizedInput) {
			return ToolResult{}, errors.New("policy returned invalid normalized input")
		}
		invocation.Call.Input = append(json.RawMessage(nil), decision.NormalizedInput...)
	}
	switch decision.Effect {
	case DecisionDeny:
		return ToolResult{}, fmt.Errorf("%w: %s", ErrPolicyDenied, decision.Reason)
	case DecisionRequireApproval:
		if grant == nil || !grant.validates(invocation, snapshot, time.Now().UTC()) {
			return ToolResult{}, &ApprovalRequiredError{Invocation: invocation, Decision: decision}
		}
	case DecisionAllow:
	default:
		return ToolResult{}, fmt.Errorf("policy returned unknown decision effect %d", decision.Effect)
	}

	lease, err := engine.registry.acquire(ctx, invocation.Call.ID, exposed.RegistrationGeneration)
	if err != nil {
		return ToolResult{}, err
	}
	defer lease.Release()

	executor, err := lease.Registration().Factory.Bind(lease.Context(), invocation.Binding)
	if err != nil {
		return ToolResult{}, fmt.Errorf("bind tool %s: %w", invocation.Call.ID.String(), err)
	}
	if executor == nil {
		return ToolResult{}, fmt.Errorf("bind tool %s: executor is nil", invocation.Call.ID.String())
	}

	base := func(executionContext context.Context, call ToolInvocation) (ToolResult, error) {
		var result ToolResult
		var executeErr error
		if invocationExecutor, ok := executor.(InvocationExecutor); ok {
			result, executeErr = invocationExecutor.ExecuteInvocation(executionContext, call)
		} else {
			result, executeErr = executor.Execute(executionContext, call.Call.Input)
		}
		if executeErr != nil {
			return ToolResult{}, fmt.Errorf("execute tool %s: %w", call.Call.ID.String(), executeErr)
		}
		return result, nil
	}
	handler := Chain(base, engine.middleware...)
	if exposed.Descriptor.DefaultTimeout > 0 {
		handler = TimeoutMiddleware(exposed.Descriptor.DefaultTimeout).Wrap(handler)
	}
	return handler(lease.Context(), invocation)
}

func validateAttachmentGeneration(snapshot ToolSetSnapshot, attachment AttachmentSnapshot) error {
	if attachment.ID == "" {
		return nil
	}
	generation, ok := snapshot.AttachmentGeneration[attachment.ID]
	if !ok {
		return fmt.Errorf("%w: attachment %q was not present", ErrStaleSnapshot, attachment.ID)
	}
	if generation != attachment.Generation {
		return fmt.Errorf("%w: attachment %q generation %d is now %d", ErrStaleSnapshot, attachment.ID, generation, attachment.Generation)
	}
	return nil
}

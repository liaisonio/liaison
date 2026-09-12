package tool

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidDescriptor = errors.New("invalid tool descriptor")
	ErrAlreadyRegistered = errors.New("tool already registered")
	ErrToolNotFound      = errors.New("tool not found")
	ErrToolUnavailable   = errors.New("tool unavailable")
	ErrStaleSnapshot     = errors.New("stale tool snapshot")
	ErrToolNotExposed    = errors.New("tool not exposed")
	ErrApprovalRequired  = errors.New("tool approval required")
	ErrPolicyDenied      = errors.New("tool policy denied")
)

type Protocol string

const (
	ProtocolAny           Protocol = "any"
	ProtocolTCP           Protocol = "tcp"
	ProtocolSSH           Protocol = "ssh"
	ProtocolWebSSH        Protocol = "web_ssh"
	ProtocolMySQL         Protocol = "mysql"
	ProtocolMariaDB       Protocol = "mariadb"
	ProtocolSQLServer     Protocol = "sqlserver"
	ProtocolOracle        Protocol = "oracle"
	ProtocolClickHouse    Protocol = "clickhouse"
	ProtocolElasticsearch Protocol = "elasticsearch"
	ProtocolOpenSearch    Protocol = "opensearch"
	ProtocolPostgreSQL    Protocol = "postgresql"
	ProtocolRedis         Protocol = "redis"
	ProtocolMongoDB       Protocol = "mongodb"
	ProtocolRDP           Protocol = "rdp"
	ProtocolVNC           Protocol = "vnc"
)

type Capability string

type OutputKind string

const (
	OutputText     OutputKind = "text"
	OutputFacts    OutputKind = "facts"
	OutputTable    OutputKind = "table"
	OutputImage    OutputKind = "image"
	OutputArtifact OutputKind = "artifact_ref"
	OutputError    OutputKind = "error"
)

type RiskLevel uint8

const (
	RiskReadOnly RiskLevel = iota
	RiskLow
	RiskMedium
	RiskHigh
	RiskCritical
)

type ApprovalMode uint8

const (
	ApprovalNever ApprovalMode = iota
	ApprovalByPolicy
	ApprovalAlways
)

type DisclosureMode uint8

const (
	DisclosureDeferred DisclosureMode = iota
	DisclosureAttachment
	DisclosureAlways
)

type ToolStatus uint8

const (
	ToolStatusActive ToolStatus = iota
	ToolStatusDisabled
	ToolStatusDraining
	ToolStatusFailed
	ToolStatusUnloaded
)

type TrustLevel uint8

const (
	TrustUntrusted TrustLevel = iota
	TrustRestricted
	TrustTrusted
	TrustBuiltin
)

type ToolID struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Version   string `json:"version"`
}

func (id ToolID) String() string {
	return id.Namespace + "." + id.Name + "@" + id.Version
}

func (id ToolID) ModelName() string {
	return id.Namespace + "." + id.Name
}

func ParseToolID(value string) (ToolID, error) {
	nameAndVersion := strings.Split(value, "@")
	if len(nameAndVersion) != 2 {
		return ToolID{}, fmt.Errorf("%w: malformed tool ID %q", ErrInvalidDescriptor, value)
	}
	nameParts := strings.Split(nameAndVersion[0], ".")
	if len(nameParts) != 2 {
		return ToolID{}, fmt.Errorf("%w: malformed tool name %q", ErrInvalidDescriptor, nameAndVersion[0])
	}
	id := ToolID{Namespace: nameParts[0], Name: nameParts[1], Version: nameAndVersion[1]}
	if !toolPartPattern.MatchString(id.Namespace) || !toolPartPattern.MatchString(id.Name) || !versionPattern.MatchString(id.Version) {
		return ToolID{}, fmt.Errorf("%w: malformed tool ID %q", ErrInvalidDescriptor, value)
	}
	return id, nil
}

type ToolSourceRef struct {
	ID    string     `json:"id"`
	Kind  string     `json:"kind"`
	Trust TrustLevel `json:"trust"`
}

type ToolDescriptor struct {
	SessionKinds   []SessionKind       `json:"session_kinds,omitempty"`
	Permission     *ResourcePermission `json:"permission,omitempty"`
	ID             ToolID              `json:"id"`
	DisplayName    string              `json:"display_name"`
	Description    string              `json:"description"`
	WhenToUse      string              `json:"when_to_use"`
	InputSchema    json.RawMessage     `json:"input_schema"`
	OutputKinds    []OutputKind        `json:"output_kinds"`
	Protocols      []Protocol          `json:"protocols"`
	Capabilities   []Capability        `json:"capabilities"`
	Tags           []string            `json:"tags,omitempty"`
	Risk           RiskLevel           `json:"risk"`
	Approval       ApprovalMode        `json:"approval"`
	Disclosure     DisclosureMode      `json:"disclosure"`
	DefaultTimeout time.Duration       `json:"default_timeout_ns"`
	Source         ToolSourceRef       `json:"source"`
}

type ToolRegistration struct {
	Descriptor ToolDescriptor
	Factory    ToolFactory
}

type ToolFactory interface {
	Bind(ctx context.Context, binding ToolBinding) (ToolExecutor, error)
}

type ToolExecutor interface {
	Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}

// InvocationExecutor is implemented when a bound executor needs the stable
// invocation ID and complete binding for idempotency or protocol-level audit.
type InvocationExecutor interface {
	ExecuteInvocation(ctx context.Context, invocation ToolInvocation) (ToolResult, error)
}

type Principal struct {
	UserID         uint
	OrganizationID uint
}

type AttachmentSnapshot struct {
	ID             string
	AccessID       uint
	ApplicationID  uint
	Protocol       Protocol
	Capabilities   []Capability
	Generation     uint64
	AccessHandleID string
}

type ToolBinding struct {
	SessionKind    SessionKind
	Principal      Principal
	Attachment     AttachmentSnapshot
	ToolSnapshotID string
}

type ToolCall struct {
	ID    ToolID
	Input json.RawMessage
}

type ToolInvocation struct {
	ID                     string
	Call                   ToolCall
	Binding                ToolBinding
	RegistrationGeneration uint64
}

type ToolResult struct {
	Kind       OutputKind
	Content    json.RawMessage
	IsError    bool
	Truncated  bool
	ArtifactID string
}

type ToolHandler func(context.Context, ToolInvocation) (ToolResult, error)

type ToolMiddleware interface {
	Wrap(next ToolHandler) ToolHandler
}

type ToolMiddlewareFunc func(next ToolHandler) ToolHandler

func (fn ToolMiddlewareFunc) Wrap(next ToolHandler) ToolHandler {
	return fn(next)
}

type DecisionEffect uint8

const (
	DecisionAllow DecisionEffect = iota
	DecisionDeny
	DecisionRequireApproval
)

type Decision struct {
	Effect          DecisionEffect
	Reason          string
	Risk            RiskLevel
	ApprovalPolicy  string
	NormalizedInput json.RawMessage
}

type PolicyEvaluator interface {
	Evaluate(ctx context.Context, invocation ToolInvocation, descriptor ToolDescriptor) (Decision, error)
}

type AllowAllPolicy struct{}

func (AllowAllPolicy) Evaluate(_ context.Context, invocation ToolInvocation, descriptor ToolDescriptor) (Decision, error) {
	if descriptor.Approval == ApprovalAlways {
		return Decision{
			Effect:          DecisionRequireApproval,
			Reason:          "tool requires explicit approval",
			Risk:            descriptor.Risk,
			NormalizedInput: invocation.Call.Input,
		}, nil
	}
	return Decision{Effect: DecisionAllow, Risk: descriptor.Risk, NormalizedInput: invocation.Call.Input}, nil
}

type ApprovalRequiredError struct {
	Invocation ToolInvocation
	Decision   Decision
}

type ApprovalGrant struct {
	ApprovalID           string
	InvocationID         string
	ToolID               ToolID
	ToolSnapshotID       string
	InputSHA256          string
	AttachmentGeneration uint64
	ExpiresAt            time.Time
}

func (grant ApprovalGrant) validates(invocation ToolInvocation, snapshot ToolSetSnapshot, now time.Time) bool {
	if grant.ApprovalID == "" || grant.InvocationID != invocation.ID || grant.ToolID != invocation.Call.ID || grant.ToolSnapshotID != snapshot.ID {
		return false
	}
	if !grant.ExpiresAt.After(now) || grant.AttachmentGeneration != invocation.Binding.Attachment.Generation {
		return false
	}
	digest := InputDigest(invocation.Call.Input)
	return digest != "" && grant.InputSHA256 == digest
}

// InputDigest matches encoding/json's RawMessage persistence representation.
// Whitespace and HTML escaping must not invalidate an otherwise unchanged grant.
// Preserve object order, duplicate keys and numbers: never reinterpret arguments.
func InputDigest(input json.RawMessage) string {
	var compact, escaped bytes.Buffer
	if err := json.Compact(&compact, input); err != nil {
		return ""
	}
	json.HTMLEscape(&escaped, compact.Bytes())
	digest := sha256.Sum256(escaped.Bytes())
	return hex.EncodeToString(digest[:])
}

func (err *ApprovalRequiredError) Error() string {
	return fmt.Sprintf("%s: %s", ErrApprovalRequired, err.Decision.Reason)
}

func (err *ApprovalRequiredError) Unwrap() error {
	return ErrApprovalRequired
}

type ExposedTool struct {
	Descriptor             ToolDescriptor
	RegistrationGeneration uint64
	EstimatedTokens        int
}

type ToolSetSnapshot struct {
	ID                   string
	CatalogGeneration    uint64
	PolicyRevision       string
	AttachmentGeneration map[string]uint64
	Tools                []ExposedTool
	CreatedAt            time.Time
}

func (snapshot ToolSetSnapshot) Find(id ToolID) (ExposedTool, bool) {
	for _, exposed := range snapshot.Tools {
		if exposed.Descriptor.ID == id {
			return exposed, true
		}
	}
	return ExposedTool{}, false
}

type DisclosureBudget struct {
	MaxAlwaysVisible int
	MaxCandidates    int
	MaxSchemas       int
	MaxTokens        int
}

func DefaultDisclosureBudget() DisclosureBudget {
	return DisclosureBudget{
		MaxAlwaysVisible: 8,
		MaxCandidates:    12,
		MaxSchemas:       16,
		MaxTokens:        6000,
	}
}

type DisclosureRequest struct {
	SessionKind    SessionKind
	Principal      Principal
	Attachments    []AttachmentSnapshot
	Primary        string
	Query          string
	Promoted       []ToolID
	RecentlyUsed   []ToolID
	PolicyRevision string
	Budget         DisclosureBudget
}

type ToolSummary struct {
	ID          ToolID     `json:"id"`
	DisplayName string     `json:"display_name"`
	Description string     `json:"description"`
	Protocols   []Protocol `json:"protocols"`
	Risk        RiskLevel  `json:"risk"`
	Tags        []string   `json:"tags,omitempty"`
}

type VisibilityFilter interface {
	AllowTool(ctx context.Context, principal Principal, attachment *AttachmentSnapshot, descriptor ToolDescriptor) (bool, error)
}

type AllowAllVisibility struct{}

func (AllowAllVisibility) AllowTool(context.Context, Principal, *AttachmentSnapshot, ToolDescriptor) (bool, error) {
	return true, nil
}

var toolPartPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)

func ValidateDescriptor(descriptor ToolDescriptor) error {
	if !toolPartPattern.MatchString(descriptor.ID.Namespace) {
		return fmt.Errorf("%w: invalid namespace %q", ErrInvalidDescriptor, descriptor.ID.Namespace)
	}
	if !toolPartPattern.MatchString(descriptor.ID.Name) {
		return fmt.Errorf("%w: invalid name %q", ErrInvalidDescriptor, descriptor.ID.Name)
	}
	if !versionPattern.MatchString(descriptor.ID.Version) {
		return fmt.Errorf("%w: invalid version %q", ErrInvalidDescriptor, descriptor.ID.Version)
	}
	if strings.TrimSpace(descriptor.Description) == "" {
		return fmt.Errorf("%w: description is required", ErrInvalidDescriptor)
	}
	if len(descriptor.Description) > 2048 || len(descriptor.WhenToUse) > 2048 {
		return fmt.Errorf("%w: descriptive text is too large", ErrInvalidDescriptor)
	}
	if len(descriptor.InputSchema) == 0 || len(descriptor.InputSchema) > 64*1024 || !json.Valid(descriptor.InputSchema) {
		return fmt.Errorf("%w: input schema must be valid JSON", ErrInvalidDescriptor)
	}
	var schema map[string]json.RawMessage
	if err := json.Unmarshal(descriptor.InputSchema, &schema); err != nil {
		return fmt.Errorf("%w: input schema must be an object: %v", ErrInvalidDescriptor, err)
	}
	if descriptor.Source.ID == "" || descriptor.Source.Kind == "" {
		return fmt.Errorf("%w: source is required", ErrInvalidDescriptor)
	}
	if descriptor.DefaultTimeout < 0 {
		return fmt.Errorf("%w: timeout cannot be negative", ErrInvalidDescriptor)
	}
	for _, kind := range descriptor.SessionKinds {
		if kind == "" || !kind.Valid() {
			return fmt.Errorf("%w: invalid session kind", ErrInvalidDescriptor)
		}
	}
	if p := descriptor.Permission; p != nil && (strings.TrimSpace(p.Resource) == "" || strings.TrimSpace(p.Action) == "") {
		return fmt.Errorf("%w: resource and action are required", ErrInvalidDescriptor)
	}
	return nil
}

func cloneDescriptor(descriptor ToolDescriptor) ToolDescriptor {
	cloned := descriptor
	cloned.SessionKinds = append([]SessionKind(nil), descriptor.SessionKinds...)
	if descriptor.Permission != nil {
		permission := *descriptor.Permission
		cloned.Permission = &permission
	}
	cloned.InputSchema = append(json.RawMessage(nil), descriptor.InputSchema...)
	cloned.OutputKinds = append([]OutputKind(nil), descriptor.OutputKinds...)
	cloned.Protocols = append([]Protocol(nil), descriptor.Protocols...)
	cloned.Capabilities = append([]Capability(nil), descriptor.Capabilities...)
	cloned.Tags = append([]string(nil), descriptor.Tags...)
	return cloned
}

func sortToolIDs(ids []ToolID) {
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
}

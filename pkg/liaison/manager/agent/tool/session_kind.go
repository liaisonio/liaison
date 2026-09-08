package tool

// SessionKind is assigned by the runtime, never by model tool arguments.
type SessionKind string

const (
	SessionAccess     SessionKind = "access"
	SessionManagement SessionKind = "management"
)

func (kind SessionKind) Valid() bool {
	return kind == "" || kind == SessionAccess || kind == SessionManagement
}

func (kind SessionKind) Effective() SessionKind {
	if kind == "" {
		return SessionAccess
	}
	return kind
}

// Empty kinds preserve the existing access-session contract. A tool must opt
// into management explicitly; ProtocolAny is not a management permission.
func descriptorMatchesSession(descriptor ToolDescriptor, kind SessionKind) bool {
	if !kind.Valid() {
		return false
	}
	if kind == "" {
		kind = SessionAccess
	}
	if len(descriptor.SessionKinds) == 0 {
		return kind == SessionAccess
	}
	for _, allowed := range descriptor.SessionKinds {
		if allowed == kind {
			return true
		}
	}
	return false
}

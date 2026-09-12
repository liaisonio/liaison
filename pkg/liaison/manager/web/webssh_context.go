package web

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// shellContext is guarded by webSSHAgentHandle.mu. It consumes OUTPUT only;
// markers are untrusted metadata, not evidence of authorization or safe input.
type shellContext struct {
	state          byte
	sequence       []byte
	cwd            string
	phase          string
	pendingCommand string
	current        *shellCommand
	commands       []shellCommand
	updated        time.Time
}

type shellCommand struct {
	Command   string `json:"command,omitempty"`
	Source    string `json:"source"`
	Directory string `json:"directory,omitempty"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Output    string `json:"output,omitempty"`
	Truncated bool   `json:"truncated"`
	started   time.Time
	finished  time.Time
	buffer    []byte
}

// OSC/CSI/DCS are streamed across arbitrary SSH reads. Oversized sequences are
// discarded until their terminator instead of buffering unbounded remote data.
func (s *shellContext) observe(data string) {
	for i := 0; i < len(data); i++ {
		b := data[i]
		switch s.state {
		case 0:
			if b == 27 {
				s.state = 1
			} else if s.current != nil && (b >= 32 || b == '\n' || b == '\t') {
				s.current.buffer = append(s.current.buffer, b)
				if len(s.current.buffer) > 4096 {
					copy(s.current.buffer, s.current.buffer[len(s.current.buffer)-2048:])
					s.current.buffer = s.current.buffer[:2048]
					s.current.Truncated = true
				}
			}
		case 1:
			switch b {
			case ']':
				s.state = 2
				s.sequence = nil
			case '[':
				s.state = 4
			case 'P', '^', '_':
				s.state = 5
			default:
				s.state = 0
			}
		case 2:
			if b == 7 {
				s.marker(string(s.sequence))
				s.sequence = nil
				s.state = 0
			} else if b == 27 {
				s.state = 3
			} else if len(s.sequence) < 8192 {
				s.sequence = append(s.sequence, b)
			} else {
				s.sequence = nil
				s.state = 5
			}
		case 3:
			if b == '\\' {
				s.marker(string(s.sequence))
				s.sequence = nil
				s.state = 0
			} else {
				s.sequence = nil
				s.state = 5
			}
		case 4:
			if b >= 0x40 && b <= 0x7e {
				s.state = 0
			}
		case 5:
			if b == 7 {
				s.state = 0
			} else if b == 27 {
				s.state = 6
			}
		case 6:
			if b == '\\' {
				s.state = 0
			} else {
				s.state = 5
			}
		}
	}
}

func (s *shellContext) marker(raw string) {
	if !strings.HasPrefix(raw, "633;") {
		return
	}
	value := strings.TrimPrefix(raw, "633;")
	now := time.Now()
	switch {
	case strings.HasPrefix(value, "P;Cwd="):
		s.cwd = decodeShellValue(strings.TrimPrefix(value, "P;Cwd="), 1024)
	case strings.HasPrefix(value, "E;"):
		// The current adapter reports a best-effort history command, not a
		// per-keystroke editor snapshot. Never claim this is exact.
		s.pendingCommand = decodeShellValue(strings.SplitN(value[2:], ";", 2)[0], 2048)
	case value == "A":
		if s.current != nil {
			s.current = nil
		}
		s.pendingCommand = ""
		s.phase = "prompt"
	case value == "B":
		s.phase = "input"
	case value == "C":
		s.current = &shellCommand{Command: s.pendingCommand, Directory: s.cwd, Source: "unknown", started: now}
		if s.pendingCommand != "" {
			s.current.Source = "shell_history_best_effort"
		}
		s.pendingCommand = ""
		s.phase = "running"
	case value == "D" || strings.HasPrefix(value, "D;"):
		if s.current != nil {
			if code, err := strconv.Atoi(strings.TrimPrefix(value, "D;")); err == nil && code >= 0 && code <= 255 {
				s.current.ExitCode = &code
			}
			s.current.ElapsedMS = now.Sub(s.current.started).Milliseconds()
			s.current.finished = now
			if len(s.current.buffer) > 2048 {
				s.current.buffer = s.current.buffer[len(s.current.buffer)-2048:]
				s.current.Truncated = true
			}
			s.current.Output = cleanShellText(string(s.current.buffer))
			s.current.buffer = nil
			s.commands = append(s.commands, *s.current)
			if len(s.commands) > 8 {
				s.commands = s.commands[len(s.commands)-8:]
			}
			s.current = nil
		}
		s.phase = "idle"
	default:
		return
	}
	s.updated = now
}

func decodeShellValue(value string, limit int) string {
	var out strings.Builder
	for i := 0; i < len(value) && out.Len() < limit; i++ {
		if value[i] == '\\' && i+1 < len(value) {
			if value[i+1] == '\\' {
				out.WriteByte('\\')
				i++
				continue
			}
			if value[i+1] == 'x' && i+3 < len(value) {
				if n, err := strconv.ParseUint(value[i+2:i+4], 16, 8); err == nil {
					out.WriteByte(byte(n))
					i += 3
					continue
				}
			}
		}
		out.WriteByte(value[i])
	}
	return cleanShellText(out.String())
}

var shellSecretAssignment = regexp.MustCompile(`(?i)((?:password|passwd|token|api[_-]?key|secret|authorization)\s*[=:]\s*)[^\s]+`)
var shellSecretFlag = regexp.MustCompile(`(?i)((?:--password|--token|--api-key|--secret)\s+)[^\s]+`)
var shellURLCredential = regexp.MustCompile(`(\w+://)[^\s/@]+:[^\s/@]+@`)

func cleanShellText(value string) string {
	value = strings.ToValidUTF8(value, "")
	value = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' || r >= 0x7f && r <= 0x9f {
			return -1
		}
		return r
	}, value)
	if strings.Contains(value, "PRIVATE KEY") {
		return "[sensitive content omitted]"
	}
	value = shellSecretAssignment.ReplaceAllString(value, "${1}[redacted]")
	value = shellSecretFlag.ReplaceAllString(value, "${1}[redacted]")
	return shellURLCredential.ReplaceAllString(value, "${1}[redacted]@")
}

func (s *shellContext) snapshot(output bool) json.RawMessage {
	now := time.Now()
	if s.updated.IsZero() || now.Sub(s.updated) > 10*time.Minute {
		s.commands = nil
		s.cwd = ""
		return json.RawMessage(`{"quality":"unavailable"}`)
	}
	commands := make([]shellCommand, 0, 3)
	for _, entry := range s.commands {
		if now.Sub(entry.finished) > 10*time.Minute {
			continue
		}
		entry.Command = cleanShellText(entry.Command)
		entry.Directory = cleanShellText(entry.Directory)
		if output {
			entry.Output = cleanShellText(entry.Output)
		} else {
			entry.Output = ""
		}
		commands = append(commands, entry)
	}
	if len(commands) > 3 {
		commands = commands[len(commands)-3:]
	}
	// This bounded JSON is generated from internal structs and cannot fail.
	value, _ := json.Marshal(struct {
		Quality   string         `json:"quality"`
		Phase     string         `json:"phase"`
		Directory string         `json:"directory,omitempty"`
		Commands  []shellCommand `json:"recent_commands"`
	}{"best_effort_untrusted", s.phase, cleanShellText(s.cwd), commands})
	if len(value) > 16384 || !utf8.Valid(value) {
		return json.RawMessage(`{"quality":"unavailable"}`)
	}
	return value
}

func (handle *webSSHAgentHandle) ShellContext(ctx context.Context, output bool) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	return handle.shell.snapshot(output), nil
}

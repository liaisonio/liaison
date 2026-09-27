package proto

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const AgentFileChunk = 128 << 10
const AgentFileLimit = 20 << 20

type AgentFileOperation struct {
	Path       string `json:"path,omitempty"`
	Name       string `json:"name,omitempty"`
	TransferID string `json:"transfer_id,omitempty"`
	Offset     int64  `json:"offset,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Data       []byte `json:"data,omitempty"`
}
type AgentFile struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Directory bool   `json:"directory,omitempty"`
	MediaType string `json:"media_type,omitempty"`
}

func (r EdgeAgentRequest) validFiles() bool {
	if len(r.Attachments) > 8 || (len(r.Attachments) > 0 && (r.Action != "send" || len(r.AccessID) != 32)) {
		return false
	}
	seen := map[string]bool{}
	for _, p := range r.Attachments {
		if p == "" || len(p) > 4096 || !utf8.ValidString(p) || strings.ContainsFunc(p, unicode.IsControl) || seen[p] {
			return false
		}
		seen[p] = true
	}
	if !strings.HasPrefix(r.Action, "file_") {
		return r.File == nil
	}
	f := r.File
	if f == nil || len(f.Path) > 4096 || !utf8.ValidString(f.Path) || strings.ContainsFunc(f.Path, unicode.IsControl) || f.Size < 0 || f.Size > AgentFileLimit || f.Offset < 0 || f.Offset > AgentFileLimit || len(f.Data) > AgentFileChunk {
		return false
	}
	switch r.Action {
	case "file_list":
		return f.Name == "" && f.TransferID == "" && len(f.Data) == 0 && f.Offset == 0 && f.Size == 0
	case "file_begin":
		return f.Path == "" && f.TransferID == "" && len(f.Data) == 0 && f.Offset == 0 && len(f.Name) > 0 && len(f.Name) <= 180 && utf8.ValidString(f.Name) && !strings.ContainsAny(f.Name, "/\\:") && !strings.ContainsFunc(f.Name, unicode.IsControl) && f.Name != "." && f.Name != ".."
	case "file_write":
		return len(f.TransferID) == 32 && f.Path == "" && f.Name == "" && f.Size == 0 && len(f.Data) > 0
	case "file_commit", "file_cancel":
		return len(f.TransferID) == 32 && f.Path == "" && f.Name == "" && f.Size == 0 && f.Offset == 0 && len(f.Data) == 0
	case "file_read":
		return f.Name == "" && f.Size == 0 && len(f.Data) == 0 && ((f.Path != "" && f.TransferID == "" && f.Offset == 0) || (f.Path == "" && len(f.TransferID) == 32))
	}
	return false
}

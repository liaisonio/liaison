package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
)

type fileTransfer struct {
	busy                   bool // Protected by Bridge.filesMu; one RPC owns the descriptor at a time.
	scope                  string
	file                   *os.File
	root                   *os.Root
	temporary, destination string
	size, offset           int64
	modified               time.Time
	touched                time.Time
}

func (t *fileTransfer) close() {
	_ = t.file.Close() // Best-effort descriptor cleanup, never completes a failed upload.
	if t.root != nil {
		_ = t.root.Remove(t.temporary) // Only this transfer's exact temporary file.
		_ = t.root.Close()
	}
}
func (b *Bridge) expireFiles(now time.Time, all bool) {
	b.filesMu.Lock()
	var expired []*fileTransfer
	for id, t := range b.files {
		if !t.busy && (all || now.Sub(t.touched) > 2*time.Minute) {
			delete(b.files, id)
			expired = append(expired, t)
		}
	}
	b.filesMu.Unlock()
	for _, t := range expired {
		t.close()
	}
}
func (b *Bridge) removeTransfer(id string, t *fileTransfer) {
	b.filesMu.Lock()
	delete(b.files, id)
	b.filesMu.Unlock()
	t.close()
}
func (b *Bridge) releaseTransfer(id string, t *fileTransfer) {
	b.filesMu.Lock()
	if b.files[id] != t {
		b.filesMu.Unlock()
		return
	}
	t.busy = false
	t.touched = time.Now()
	closed := b.ctx.Err() != nil
	if closed {
		delete(b.files, id)
	}
	b.filesMu.Unlock()
	if closed {
		t.close()
	}
}
func (b *Bridge) registerTransfer(id string, t *fileTransfer) bool {
	b.filesMu.Lock()
	defer b.filesMu.Unlock()
	if len(b.files) >= 8 || b.ctx.Err() != nil {
		return false
	}
	if b.files == nil {
		b.files = make(map[string]*fileTransfer)
	}
	b.files[id] = t
	return true
}
func fileToken() string {
	var v [16]byte
	if _, err := rand.Read(v[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(v[:])
}
func filePath(p string) (string, bool) {
	if p == "" {
		p = "."
	}
	if strings.Contains(p, "\\") || !filepath.IsLocal(p) {
		return "", false
	}
	for _, part := range strings.Split(filepath.ToSlash(p), "/") {
		if strings.HasPrefix(part, ".upload-") {
			return "", false
		}
	}
	return filepath.Clean(p), true
}
func attachmentName(p string) string {
	name := filepath.Base(p)
	if len(name) > 33 && name[32] == '-' {
		return name[33:]
	}
	return name
}

// Content access is narrower than directory discovery: only the bound session
// project, never the entire OS user's home. os.Root pins traversal boundaries.
func (b *Bridge) handleFiles(ctx context.Context, input proto.EdgeAgentRPCRequest) proto.EdgeAgentResult {
	req := input.Request
	f := req.File
	scope := bindingKey(input.ActorID, req.AccessID, req.SessionID)
	b.mu.Lock()
	binding, ok := b.bindings[scope]
	b.mu.Unlock()
	savedRoot, rootErr := filepath.EvalSymlinks(input.ProjectRoot)
	if !ok || rootErr != nil || binding.AccessProject != savedRoot {
		return result("not_found")
	}
	if ctx.Err() != nil {
		return result("unavailable")
	}
	if f.TransferID != "" {
		b.filesMu.Lock()
		t := b.files[f.TransferID]
		if t == nil || t.scope != scope {
			b.filesMu.Unlock()
			return result("not_found")
		}
		if t.busy {
			b.filesMu.Unlock()
			return result("busy")
		}
		t.busy = true
		b.filesMu.Unlock()
		defer b.releaseTransfer(f.TransferID, t)
		fail := func() proto.EdgeAgentResult {
			b.removeTransfer(f.TransferID, t)
			return result("invalid_request")
		}
		switch req.Action {
		case "file_cancel":
			b.removeTransfer(f.TransferID, t)
			return result("ok")
		case "file_write":
			if t.root == nil || f.Offset != t.offset || t.offset+int64(len(f.Data)) > t.size {
				return fail()
			}
			n, err := t.file.Write(f.Data)
			if err != nil || n != len(f.Data) {
				return fail()
			}
			t.offset += int64(n)
			out := result("ok")
			out.FileOffset = t.offset
			return out
		case "file_commit":
			if t.root == nil || t.offset != t.size {
				return fail()
			}
			mediaType := ""
			if isImageName(t.destination) {
				if _, err := t.file.Seek(0, io.SeekStart); err != nil {
					return fail()
				}
				_, kind, err := readImage(t.file)
				if err != nil {
					return fail()
				}
				mediaType = kind
			}
			if err := t.file.Sync(); err != nil {
				return fail()
			}
			if err := t.file.Close(); err != nil {
				return fail()
			}
			if ctx.Err() != nil {
				return fail()
			}
			// Random destination, not a caller-supplied project file. Link is atomic and
			// fails if a destination already exists, unlike a replacing rename.
			if err := t.root.Link(t.temporary, t.destination); err != nil {
				return fail()
			}
			out := result("ok")
			out.File = &proto.AgentFile{Name: attachmentName(t.destination), Path: filepath.ToSlash(t.destination), Size: t.size, MediaType: mediaType}
			b.removeTransfer(f.TransferID, t)
			return out
		case "file_read":
			if t.root != nil {
				return fail()
			}
			return b.readFileChunk(ctx, f.TransferID, t, f.Offset)
		default:
			return result("invalid_request")
		}
	}
	root, err := os.OpenRoot(binding.Project)
	if err != nil {
		return result("unavailable")
	}
	defer root.Close()
	p, ok := filePath(f.Path)
	if !ok {
		return result("invalid_request")
	}
	switch req.Action {
	case "file_list":
		dir, err := openAgentFile(root, p)
		if err != nil {
			return result("invalid_request")
		}
		defer dir.Close()
		entries, err := dir.ReadDir(500)
		if err != nil && err != io.EOF {
			return result("invalid_request")
		}
		out := result("ok")
		out.FilesAvailable = true
		out.Directory = filepath.ToSlash(p)
		out.Truncated = len(entries) == 500
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".upload-") {
				continue
			}
			if ctx.Err() != nil {
				return result("unavailable")
			}
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			info, err := entry.Info()
			if err != nil || (!info.IsDir() && !info.Mode().IsRegular()) {
				continue
			}
			out.Files = append(out.Files, proto.AgentFile{Name: entry.Name(), Path: filepath.ToSlash(filepath.Join(p, entry.Name())), Size: info.Size(), Directory: info.IsDir()})
		}
		sort.Slice(out.Files, func(i, j int) bool {
			if out.Files[i].Directory != out.Files[j].Directory {
				return out.Files[i].Directory
			}
			return out.Files[i].Name < out.Files[j].Name
		})
		return out
	case "file_begin":
		if isImageName(f.Name) && f.Size > 4<<20 {
			return result("invalid_request")
		}
		id := fileToken()
		if id == "" {
			return result("unavailable")
		}
		dir := filepath.Join(".liaison-attachments", req.SessionID)
		if err := root.MkdirAll(dir, 0700); err != nil {
			return result("unavailable")
		}
		temporary := filepath.Join(dir, ".upload-"+id)
		file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			return result("unavailable")
		}
		// Own a second root descriptor for the transfer beyond this RPC.
		owned, err := root.OpenRoot(".")
		if err != nil {
			file.Close()
			root.Remove(temporary)
			return result("unavailable")
		}
		t := &fileTransfer{scope: scope, file: file, root: owned, temporary: temporary, destination: filepath.Join(dir, id+"-"+f.Name), size: f.Size, touched: time.Now()}
		if !b.registerTransfer(id, t) {
			t.close()
			return result("busy")
		}
		out := result("ok")
		out.TransferID = id
		return out
	case "file_read":
		file, err := openAgentFile(root, p)
		if err != nil {
			return result("invalid_request")
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > proto.AgentFileLimit || f.Offset != 0 {
			file.Close()
			return result("invalid_request")
		}
		id := fileToken()
		if id == "" {
			file.Close()
			return result("unavailable")
		}
		t := &fileTransfer{busy: true, scope: scope, file: file, size: info.Size(), modified: info.ModTime(), touched: time.Now()}
		if !b.registerTransfer(id, t) {
			t.close()
			return result("busy")
		}
		defer b.releaseTransfer(id, t)
		out := b.readFileChunk(ctx, id, t, 0)
		out.File = &proto.AgentFile{Name: filepath.Base(p), Path: filepath.ToSlash(p), Size: info.Size()}
		return out
	}
	return result("invalid_request")
}
func (b *Bridge) readFileChunk(ctx context.Context, id string, t *fileTransfer, offset int64) proto.EdgeAgentResult {
	fail := func() proto.EdgeAgentResult { b.removeTransfer(id, t); return result("invalid_request") }
	info, err := t.file.Stat()
	if err != nil || info.Size() != t.size || !info.ModTime().Equal(t.modified) || offset != t.offset || ctx.Err() != nil {
		return fail()
	}
	remaining := min(int64(proto.AgentFileChunk), t.size-t.offset)
	data := make([]byte, int(remaining))
	if _, err := io.ReadFull(t.file, data); err != nil {
		return fail()
	}
	after, err := t.file.Stat()
	if err != nil || after.Size() != t.size || !after.ModTime().Equal(t.modified) {
		return fail()
	}
	t.offset += remaining
	out := result("ok")
	out.FileData = data
	out.FileOffset = t.offset
	out.TransferID = id
	out.FileDone = t.offset == t.size
	if out.FileDone {
		b.removeTransfer(id, t)
	}
	return out
}

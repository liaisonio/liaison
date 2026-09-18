// Package smbfiles reads a single remote SMB share over a supplied tunnel.
// It never mounts a local filesystem or opens a second network destination.
package smbfiles

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	smb "github.com/cloudsoda/go-smb2"
)

const TransferLimit = 64 * 1024 * 1024
const PreviewLimit = 256 * 1024
const EntryLimit = 10000

var ErrInvalid = errors.New("invalid SMB path or connection")
var ErrLimit = errors.New("SMB result exceeds limit")
var ErrRead = errors.New("SMB operation failed; check permissions and reconnect")

type Entry struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	Mode       string    `json:"mode"`
	Directory  bool      `json:"directory"`
	Symlink    bool      `json:"symlink"`
	ModifiedAt time.Time `json:"modified_at"`
}

type Files struct {
	conn  net.Conn
	share *smb.Share
}

func ValidShare(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 && utf8.ValidString(name) && !strings.ContainsAny(name, "/\\:\x00\r\n*?\"<>|")
}

// Normalize accepts only root-relative paths, never UNC, drive names, alternate
// streams, traversal, wildcards or ambiguous Win32 trailing dots/spaces.
func Normalize(value string) (string, error) {
	if len(value) > 4096 || !utf8.ValidString(value) || strings.HasPrefix(value, "//") || strings.ContainsAny(value, "\\:\x00\r\n*?\"<>|") {
		return "", ErrInvalid
	}
	value = strings.TrimPrefix(value, "/")
	value = strings.TrimSuffix(value, "/")
	if value == "" {
		return ".", nil
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return "", ErrInvalid
		}
	}
	return strings.ReplaceAll(value, "/", "\\"), nil
}

// New takes ownership of conn even on failure. NTLM password authentication and
// message signing are required; negotiated SMB encryption is not claimed as TLS.
func New(ctx context.Context, conn net.Conn, host, user, password, domain, share string) (*Files, error) {
	if conn == nil {
		return nil, ErrInvalid
	}
	if !ValidShare(share) || user == "" || strings.ContainsAny(user+domain, "\x00\r\n") {
		conn.Close()
		return nil, ErrInvalid
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	d := smb.Dialer{Negotiator: smb.Negotiator{RequireMessageSigning: true}, Initiator: &smb.NTLMInitiator{User: user, Password: password, Domain: domain}}
	session, err := d.DialConn(ctx, conn, host)
	if err != nil {
		conn.Close()
		return nil, ErrRead
	}
	fs, err := session.WithContext(ctx).Mount(share)
	if err != nil || ctx.Err() != nil {
		conn.Close()
		return nil, ErrRead
	}
	return &Files{conn: conn, share: fs}, nil
}
func (f *Files) Close() error { return f.conn.Close() }

// check rejects visible reparse/symlink components. The single negotiated tree
// remains the boundary even if a server changes a path between requests.
func (f *Files) check(ctx context.Context, p string) error {
	current := ""
	for _, part := range strings.Split(p, "\\") {
		if current != "" {
			current += "\\"
		}
		current += part
		info, err := f.share.WithContext(ctx).Lstat(current)
		if err != nil {
			return ErrRead
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalid
		}
	}
	return nil
}
func (f *Files) List(ctx context.Context, value string) ([]Entry, error) {
	p, err := Normalize(value)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { f.conn.Close() })
	defer stop()
	if err = f.check(ctx, p); err != nil {
		return nil, err
	}
	dir, err := f.share.WithContext(ctx).Open(p)
	if err != nil {
		return nil, ErrRead
	}
	defer dir.Close()
	infos, err := dir.Readdir(EntryLimit + 1)
	if err != nil && err != io.EOF {
		return nil, ErrRead
	}
	if len(infos) > EntryLimit {
		return nil, ErrLimit
	}
	result := make([]Entry, 0, len(infos))
	for _, info := range infos {
		if info.Name() == "." || info.Name() == ".." {
			continue
		}
		result = append(result, Entry{Name: info.Name(), Size: info.Size(), Mode: info.Mode().String(), Directory: info.IsDir(), Symlink: info.Mode()&os.ModeSymlink != 0, ModifiedAt: info.ModTime()})
	}
	return result, nil
}
func (f *Files) Read(ctx context.Context, value string, limit int64) ([]byte, error) {
	p, err := Normalize(value)
	if err != nil || p == "." {
		return nil, ErrInvalid
	}
	stop := context.AfterFunc(ctx, func() { f.conn.Close() })
	defer stop()
	if err = f.check(ctx, p); err != nil {
		return nil, err
	}
	file, err := f.share.WithContext(ctx).Open(p)
	if err != nil {
		return nil, ErrRead
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, ErrInvalid
	}
	if info.Size() > limit {
		return nil, ErrLimit
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, ErrRead
	}
	if int64(len(data)) > limit {
		return nil, ErrLimit
	}
	return data, nil
}
func (f *Files) Preview(ctx context.Context, value string) (string, error) {
	data, err := f.Read(ctx, value, PreviewLimit)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
		return "", ErrInvalid
	}
	return string(data), nil
}

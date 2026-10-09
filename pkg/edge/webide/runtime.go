package webide

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio"
)

var ErrUnavailable = errors.New("IDE instance unavailable")

type record struct {
	proto.WebIDEInstance
	Owner   string `json:"owner"`
	PID     int32  `json:"pid"`
	Created int64  `json:"process_created"`
	Socket  string `json:"socket"`
	DataDir string `json:"data_dir"`
}

// Service is scoped to one OS user. An authenticated Manager must additionally
// authorize connector ownership and access permissions before invoking it.
type Service struct {
	mu            sync.Mutex
	operations    [32]sync.Mutex
	root          string
	records       map[string]*record
	installing    bool
	installStatus string
	installer     Installer
}

func New(root string) (*Service, error) {
	return NewWithInstaller(root, PackageInstaller{})
}

// NewWithInstaller keeps deployment-specific package acquisition outside the
// instance lifecycle. Production adapters must preserve package verification.
func NewWithInstaller(root string, installer Installer) (*Service, error) {
	if installer == nil {
		return nil, errors.New("IDE installer is required")
	}
	if !filepath.IsAbs(root) {
		return nil, errors.New("IDE state directory must be absolute")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	if err := privateDirectory(root); err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	root = canonical
	s := &Service{root: root, records: map[string]*record{}, installer: installer}
	if err := s.restoreInstallStatus(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "instances.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("IDE state too large")
	}
	if err = json.Unmarshal(data, &s.records); err != nil {
		return nil, fmt.Errorf("read IDE state: %w", err)
	}
	if s.records == nil {
		s.records = map[string]*record{}
	}
	if len(s.records) > 128 {
		return nil, errors.New("too many IDE records")
	}
	for id, r := range s.records {
		if r == nil || r.ID != id || !validID(id) || r.DataDir != filepath.Join(root, id, "data") || (r.Socket != filepath.Join(root, id, "s") && r.Socket != s.socketPath(id)) {
			return nil, errors.New("invalid IDE state")
		}
	}
	return s, nil
}

// socketPath retains short legacy paths. Long state roots use a stable private
// directory directly below the OS temporary root, never a caller-supplied path.
func (s *Service) socketPath(id string) string {
	path := filepath.Join(s.root, id, "s")
	if len(path) <= 100 {
		return path
	}
	sum := sha256.Sum256([]byte(s.root + "/" + id))
	return filepath.Join("/tmp", fmt.Sprintf("liaison-ide-%d-%x", os.Getuid(), sum[:16]), "s")
}

func validID(id string) bool {
	if len(id) != 24 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func (s *Service) save() error {
	data, err := json.Marshal(s.records)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.root, ".state-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(s.root, "instances.json"))
}

func (s *Service) discover(ctx context.Context) []proto.WebIDEInstallation {
	paths := []struct{ path, source string }{{filepath.Join(s.root, "packages", packageName(), "bin", "code-server"), "managed"}}
	if p, err := exec.LookPath("code-server"); err == nil {
		paths = append(paths, struct{ path, source string }{p, "existing"})
	}
	for _, p := range []string{"/opt/homebrew/bin/code-server", "/usr/local/bin/code-server", "/usr/bin/code-server"} {
		paths = append(paths, struct{ path, source string }{p, "existing"})
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, struct{ path, source string }{filepath.Join(home, ".local", "bin", "code-server"), "existing"})
	}
	out := []proto.WebIDEInstallation{}
	seen := map[string]bool{}
	for _, p := range paths {
		if ctx.Err() != nil {
			break
		}
		resolved, err := filepath.EvalSymlinks(p.path)
		if p.source == "managed" {
			resolved, err = s.managedProgram()
		}
		if err != nil || seen[resolved] {
			continue
		}
		seen[resolved] = true
		if err = trustedProgram(resolved); err != nil {
			continue
		}
		// Discover metadata without executing arbitrary PATH candidates.
		digest := sha256.Sum256([]byte(resolved))
		version := ""
		if p.source == "managed" {
			version = Release
		}
		out = append(out, proto.WebIDEInstallation{ID: hex.EncodeToString(digest[:12]), Version: version, Source: p.source, Path: resolved})
	}
	return out
}

// A directory entry alone is not a usable installation. In particular, never
// advertise a broken link, non-executable file or executable outside the package.
func (s *Service) managedProgram() (string, error) {
	root := filepath.Join(s.root, "packages", packageName())
	program, err := filepath.EvalSymlinks(filepath.Join(root, "bin", "code-server"))
	if err != nil {
		return "", err
	}
	if !within(root, program) {
		return "", errors.New("IDE executable outside managed package")
	}
	if err := trustedProgram(program); err != nil {
		return "", err
	}
	return program, nil
}

func (s *Service) Handle(ctx context.Context, q proto.WebIDERequest) (out proto.WebIDEResult) {
	out = proto.WebIDEResult{Version: 1, Status: "invalid_request", Platform: platform(), CanLaunch: canLaunch()}
	if !q.Valid() {
		return out
	}
	started := time.Now()
	defer func() {
		if q.RequestID != "" {
			slog.InfoContext(ctx, "IDE connector operation", "request_id", q.RequestID, "stage", "connector", "action", q.Action, "status", out.Status, "duration_ms", time.Since(started).Milliseconds())
		}
	}()
	if q.Action == "directories" {
		return directories(ctx, q.Directory)
	}
	// Serialize mutations of the same access without holding the registry lock
	// during cold startup. A fixed stripe count bounds memory for invalid IDs.
	if q.Action == "start" || q.Action == "stop" {
		key := sha256.Sum256([]byte(q.OwnerID + ":" + q.AccessID))
		operation := &s.operations[int(key[0])%len(s.operations)]
		operation.Lock()
		defer operation.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out.Status = "ok"
	switch q.Action {
	case "discover":
		out.Installations = s.discover(ctx)
		if s.installing {
			out.Status = "installing"
		} else if s.installStatus != "" {
			out.Status = s.installStatus
			// Discovery is read-only. A verified offline installation may have
			// repaired the package since the last failed download; report its
			// current availability without rewriting the installation history.
			if out.Status == "install_failed" {
				if _, err := s.managedProgram(); err == nil {
					out.Status = "ok"
				}
			}
		}
	case "install":
		if !canLaunch() {
			out.Status = "non_root_required"
			break
		}
		if s.installing {
			out.Status = "installing"
			break
		}
		if _, err := os.Lstat(filepath.Join(s.root, "packages", packageName())); err == nil {
			if _, err := s.managedProgram(); err != nil {
				s.installStatus = "install_failed"
				out.Status = s.installStatus
				if err := s.saveInstallStatus(s.installStatus); err != nil {
					out.Status = "state_failed"
				}
				break
			}
			if err := s.saveInstallStatus("ok"); err != nil {
				out.Status = "state_failed"
				break
			}
			s.installStatus = "ok"
			out.Installations = s.discover(ctx)
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			out.Status = "install_failed"
			break
		}
		s.installing = true
		s.installStatus = ""
		if err := s.saveInstallStatus("installing"); err != nil {
			s.installing = false
			out.Status = "state_failed"
			break
		}
		out.Status = "installing"
		go s.install()
	case "instances":
		for _, r := range s.records {
			if r.Owner == q.OwnerID {
				v := r.WebIDEInstance
				v.Status = "stopped"
				if alive(r) {
					v.Status = "running"
					if r.Status == "starting" {
						v.Status = "starting"
					}
				}
				out.Instances = append(out.Instances, v)
			}
		}
		sort.Slice(out.Instances, func(i, j int) bool { return out.Instances[i].StartedAt > out.Instances[j].StartedAt })
	case "start":
		if !canLaunch() {
			out.Status = "non_root_required"
			break
		}
		r, err := s.start(ctx, q)
		if err != nil {
			out.Status = "start_failed"
			break
		}
		out.Instances = []proto.WebIDEInstance{r.WebIDEInstance}
	case "stop":
		r := s.records[q.InstanceID]
		if r == nil || r.Owner != q.OwnerID || r.AccessID != q.AccessID {
			out.Status = "not_found"
			break
		}
		if err := stopProcess(ctx, r); err != nil {
			out.Status = "stop_failed"
			break
		}
		r.Status = "stopped"
		r.PID = 0
		r.Created = 0
		if err := s.save(); err != nil {
			out.Status = "state_failed"
			break
		}
		out.Instances = []proto.WebIDEInstance{r.WebIDEInstance}
	}
	return out
}

func (s *Service) install() {
	status := "install_failed"
	defer func() {
		if recover() != nil {
			status = "install_failed"
		}
		s.mu.Lock()
		s.installing = false
		s.installStatus = status
		if err := s.saveInstallStatus(status); err != nil {
			s.installStatus = "state_failed"
		}
		s.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if _, err := s.installer.Install(ctx, filepath.Join(s.root, "packages")); err == nil {
		if _, err := s.managedProgram(); err == nil {
			status = "ok"
		}
	}
}

func (s *Service) start(ctx context.Context, q proto.WebIDERequest) (*record, error) {
	project := ""
	var err error
	if q.Project != "" {
		project, err = filepath.EvalSymlinks(q.Project)
		if err != nil || !filepath.IsAbs(project) {
			return nil, errors.New("invalid project")
		}
		info, statErr := os.Stat(project)
		if statErr != nil || !info.IsDir() {
			return nil, errors.New("project directory unavailable")
		}
	}
	var installation *proto.WebIDEInstallation
	for _, i := range s.discover(ctx) {
		if i.ID == q.InstallationID {
			x := i
			installation = &x
			break
		}
	}
	if installation == nil {
		return nil, errors.New("installation unavailable")
	}
	var current *record
	running := 0
	for _, r := range s.records {
		live := alive(r)
		if live {
			running++
		}
		if r.Owner == q.OwnerID && r.AccessID == q.AccessID && r.Project == project {
			if r.ApplicationID != "" && q.ApplicationID != "" && r.ApplicationID != q.ApplicationID {
				return nil, errors.New("application conflict")
			}
			if r.InstallationID != q.InstallationID {
				return nil, errors.New("installation conflict")
			}
			current = r
			if q.ApplicationID != "" {
				r.ApplicationID = q.ApplicationID
			}
			if live {
				r.Status = "running"
				if err = s.save(); err != nil {
					return nil, err
				}
				return r, nil
			}
		}
	}
	if running >= 8 {
		return nil, errors.New("IDE instance capacity exceeded")
	}
	newProfile := current == nil
	if current == nil {
		if len(s.records) >= 128 {
			return nil, errors.New("IDE profile capacity exceeded")
		}
		var random [12]byte
		if _, err = rand.Read(random[:]); err != nil {
			return nil, err
		}
		id := hex.EncodeToString(random[:])
		current = &record{Owner: q.OwnerID, WebIDEInstance: proto.WebIDEInstance{ID: id, AccessID: q.AccessID, InstallationID: q.InstallationID, ApplicationID: q.ApplicationID}}
		current.DataDir = filepath.Join(s.root, id, "data")
		current.Socket = s.socketPath(id)
	}
	// Stopped legacy profiles may have an overlong socket path.
	current.Socket = s.socketPath(current.ID)
	if len(current.Socket) > 100 {
		return nil, errors.New("IDE socket path too long")
	}
	base := filepath.Join(s.root, current.ID)
	if err = os.MkdirAll(base, 0700); err != nil {
		return nil, err
	}
	if err = privateDirectory(base); err != nil {
		return nil, err
	}
	socketDir := filepath.Dir(current.Socket)
	if socketDir != base {
		if err = os.Mkdir(socketDir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if err = privateDirectory(socketDir); err != nil {
			return nil, err
		}
	}
	if newProfile && installation.Source == "managed" {
		if err = seedManagedProfile(ctx, current.DataDir, filepath.Join(s.root, "packages", packageName())); err != nil {
			return nil, err
		}
	}
	// Keep both sockets short: the upstream default session socket under the
	// data directory can exceed macOS/Linux Unix socket path limits even when
	// our HTTP socket fits. Both live in the same private instance directory.
	sessionSocket := filepath.Join(socketDir, "i")
	for _, socket := range []string{current.Socket, sessionSocket} {
		if st, e := os.Lstat(socket); e == nil {
			if st.Mode()&os.ModeSocket == 0 {
				return nil, errors.New("unexpected socket path")
			}
			if err = os.Remove(socket); err != nil {
				return nil, err
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			return nil, e
		}
	}
	config := filepath.Join(base, "config.yaml")
	if err = os.WriteFile(config, []byte("auth: none\ncert: false\n"), 0600); err != nil {
		return nil, err
	}
	args := []string{"--config", config, "--auth", "none", "--socket", current.Socket, "--session-socket", sessionSocket, "--socket-mode", "0600", "--user-data-dir", current.DataDir, "--extensions-dir", filepath.Join(base, "extensions"), "--disable-proxy", "--disable-telemetry", "--disable-update-check", "--disable-getting-started-override"}
	if project != "" {
		args = append(args, project)
	}
	cmd := exec.Command(installation.Path, args...)
	cmd.Dir = base
	if project != "" {
		cmd.Dir = project
	}
	cmd.Env = processEnvironment()
	prepareProcess(cmd)
	// stdout/stderr are not sent to Manager or retained without bounds. Native
	// IDE logs are available under its user-data directory for local diagnosis.
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	go func() { _ = cmd.Wait() }() // Reap only our child; exit is observed via identity/health checks.
	current.PID = int32(cmd.Process.Pid)
	created, e := processCreated(current.PID)
	current.Created = created
	if e != nil {
		cmd.Process.Kill()
		return nil, e
	}
	current.Project = project
	current.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	current.Status = "starting"
	s.records[current.ID] = current
	if err = s.save(); err != nil {
		stopProcess(context.Background(), current)
		delete(s.records, current.ID)
		return nil, err
	}
	snapshot := *current
	s.mu.Unlock()
	err = waitReady(ctx, &snapshot)
	s.mu.Lock()
	if err != nil {
		stopCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
		stopErr := stopProcess(stopCtx, current)
		done()
		current.Status = "failed"
		return nil, errors.Join(err, stopErr, s.save())
	}
	current.Status = "running"
	return current, s.save()
}

func waitReady(ctx context.Context, current *record) error {
	ready, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	transport := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(c, "unix", current.Socket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, e := http.NewRequestWithContext(ready, http.MethodGet, "http://localhost/healthz", nil)
		if e == nil {
			resp, e := client.Do(request)
			if e == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
		select {
		case <-ready.Done():
			return ready.Err()
		case <-ticker.C:
		}
	}
}

func alive(r *record) bool {
	if r.PID <= 0 || r.Created == 0 {
		return false
	}
	created, err := processCreated(r.PID)
	if err != nil || created != r.Created {
		return false
	}
	args, err := processArgs(r.PID)
	if err != nil {
		return false
	}
	for i, a := range args {
		if a == "--user-data-dir" && i+1 < len(args) && args[i+1] == r.DataDir {
			return true
		}
	}
	return false
}

func (s *Service) Dial(ctx context.Context, owner, access, id string) (net.Conn, error) {
	s.mu.Lock()
	r := s.records[id]
	if r == nil || r.Owner != owner || r.AccessID != access || !alive(r) {
		s.mu.Unlock()
		return nil, ErrUnavailable
	}
	socket := r.Socket
	s.mu.Unlock()
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socket)
}

func processEnvironment() []string {
	out := []string{}
	for _, key := range []string{"HOME", "PATH", "LANG", "LC_ALL", "LC_CTYPE", "TMPDIR", "TZ", "SHELL", "USER", "LOGNAME"} {
		if value, ok := os.LookupEnv(key); ok {
			out = append(out, key+"="+value)
		}
	}
	return out
}

func directories(ctx context.Context, dir string) proto.WebIDEResult {
	out := proto.WebIDEResult{Version: 1, Status: "invalid_request", Platform: platform(), CanLaunch: canLaunch()}
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	if !filepath.IsAbs(dir) {
		return out
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return out
	}
	f, err := os.Open(resolved)
	if err != nil {
		return out
	}
	defer f.Close()
	entries, err := f.ReadDir(2000)
	if err != nil && err != io.EOF {
		return out
	}
	out.Status = "ok"
	out.Directory = resolved
	out.Truncated = len(entries) == 2000
	for _, e := range entries {
		if ctx.Err() != nil {
			out.Status = "unavailable"
			return out
		}
		if e.IsDir() {
			out.Directories = append(out.Directories, proto.WebIDEDirectory{Name: e.Name(), Path: filepath.Join(resolved, e.Name())})
		}
	}
	sort.Slice(out.Directories, func(i, j int) bool {
		return strings.ToLower(out.Directories[i].Name) < strings.ToLower(out.Directories[j].Name)
	})
	return out
}

type registrar interface {
	RegisterRPCHandler(string, func(context.Context, geminio.Request, geminio.Response)) error
}

func (s *Service) Register(r registrar) error {
	return r.RegisterRPCHandler(proto.RPCWebIDE, func(ctx context.Context, req geminio.Request, rsp geminio.Response) {
		if len(req.Data()) > 16<<10 {
			rsp.SetError(errors.New("invalid IDE request"))
			return
		}
		var input proto.WebIDERequest
		d := json.NewDecoder(bytes.NewReader(req.Data()))
		d.DisallowUnknownFields()
		if d.Decode(&input) != nil || d.Decode(new(any)) != io.EOF {
			rsp.SetError(errors.New("invalid IDE request"))
			return
		}
		data, err := json.Marshal(s.Handle(ctx, input))
		if err != nil {
			rsp.SetError(errors.New("IDE response unavailable"))
			return
		}
		rsp.SetData(data)
	})
}

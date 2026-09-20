package lifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
)

type workerJob struct {
	Identity    Identity
	Plan        Plan
	Command     proto.UninstallCommand
	Digests     map[string]string
	InsecureTLS bool
}

func digestFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func validateCommand(cmd proto.UninstallCommand, hosts []string, now time.Time) error {
	if !instancePattern.MatchString(cmd.TaskID) || !instancePattern.MatchString(cmd.RuntimeID) || len(cmd.CallbackToken) != 64 || cmd.ExpiresAt <= now.Unix() || cmd.ExpiresAt > now.Add(5*time.Minute).Unix() {
		return ErrUnsupported
	}
	if _, err := hex.DecodeString(cmd.CallbackToken); err != nil {
		return ErrUnsupported
	}
	u, err := url.Parse(cmd.CallbackURL)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/api/v1/edge-uninstall-results/"+cmd.TaskID {
		return ErrUnsupported
	}
	for _, addr := range hosts {
		host, _, err := net.SplitHostPort(addr)
		if err == nil && strings.EqualFold(host, u.Hostname()) {
			return nil
		}
	}
	return ErrUnsupported
}

// Prepare launches a service-manager-owned one-shot helper, not a child that
// dies when its parent service stops. Paths originate solely from the local plan.
func Prepare(ctx context.Context, id Identity, command proto.UninstallCommand, hosts []string, insecureTLS bool, run Command) error {
	if err := validateCommand(command, hosts, time.Now()); err != nil {
		return err
	}
	plan, err := Check(ctx, id, run)
	if err != nil {
		return err
	}
	if plan.InstanceID != command.InstanceID {
		return ErrUnsupported
	}
	job := workerJob{Identity: id, Plan: plan, Command: command, InsecureTLS: insecureTLS, Digests: map[string]string{}}
	for _, path := range []string{plan.ServiceFile, plan.Executable, plan.Config} {
		sum, err := digestFile(path)
		if err != nil {
			return err
		}
		job.Digests[path] = sum
	}
	dir, err := os.MkdirTemp("", "liaison-uninstall-")
	if err != nil {
		return err
	}
	executable := filepath.Join(dir, "worker")
	jobPath := filepath.Join(dir, "job.json")
	cleanup := func() { // Exact newly created files only; never recursively remove a directory.
		for _, path := range []string{jobPath, executable, dir} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) { /* Retain failed-cleanup files for manual diagnosis. */
			}
		}
	}
	source, err := os.Open(plan.Executable)
	if err != nil {
		cleanup()
		return err
	}
	target, err := os.OpenFile(executable, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		source.Close()
		cleanup()
		return err
	}
	_, copyErr := io.Copy(target, source)
	source.Close()
	closeErr := target.Close()
	if copyErr != nil {
		cleanup()
		return copyErr
	}
	if closeErr != nil {
		cleanup()
		return closeErr
	}
	if sum, err := digestFile(executable); err != nil || sum != job.Digests[plan.Executable] {
		cleanup()
		return ErrUnsupported
	}
	data, err := json.Marshal(job)
	if err != nil {
		cleanup()
		return err
	}
	if err = os.WriteFile(jobPath, data, 0600); err != nil {
		cleanup()
		return err
	}
	if id.OS == "linux" {
		args := []string{"--quiet", "--collect", "--unit=liaison-uninstall-" + command.TaskID, "--property=Type=exec", "--property=RuntimeMaxSec=90", executable, "--edge-uninstall-worker", jobPath}
		if plan.Kind == "systemd-user" {
			args = append([]string{"--user"}, args...)
		}
		_, err = run(ctx, "/usr/bin/systemd-run", args...)
	} else {
		_, err = run(ctx, "/bin/launchctl", "submit", "-l", "com.liaison.uninstall."+command.TaskID, "--", executable, "--edge-uninstall-worker", jobPath)
	}
	if err != nil {
		cleanup()
		return err
	}
	return nil
}

// Apply performs the destructive stage through injected operations. Every target
// is revalidated before stopping, and every file is checked again before removal.
func apply(ctx context.Context, job workerJob, check func(context.Context, Identity, Command) (Plan, error), run Command, validate func(string, int) error, remove func(string) error) error {
	if job.Command.ExpiresAt <= time.Now().Unix() {
		return ErrUnsupported
	}
	plan, err := check(ctx, job.Identity, run)
	if err != nil {
		return err
	}
	if plan != job.Plan || plan.InstanceID != job.Command.InstanceID {
		return ErrUnsupported
	}
	paths := []string{plan.ServiceFile, plan.Config, plan.Executable}
	for _, path := range paths {
		sum, err := digestFile(path)
		if err != nil {
			return err
		}
		if sum != job.Digests[path] {
			return ErrUnsupported
		}
	}
	if plan.Kind == "systemd" || plan.Kind == "systemd-user" {
		if _, err = run(ctx, "/usr/bin/systemctl", systemdArgs(plan, "disable", "--now", plan.Service)...); err != nil {
			return err
		}
	} else if plan.Kind == "launchd" {
		if _, err = run(ctx, "/bin/launchctl", "bootout", fmt.Sprintf("gui/%d/%s", job.Identity.UID, plan.Service)); err != nil {
			return err
		}
	} else {
		return ErrUnsupported
	}
	// A successful stop request is not proof that the original process exited.
	if err = waitProcessGone(ctx, job.Identity.PID, run); err != nil {
		return err
	}
	for _, path := range paths {
		if err := validate(path, job.Identity.UID); err != nil {
			return err
		}
		sum, err := digestFile(path)
		if err != nil {
			return err
		}
		if sum != job.Digests[path] {
			return ErrUnsupported
		}
		if err = remove(path); err != nil {
			return err
		}
	}
	if plan.Kind == "systemd" || plan.Kind == "systemd-user" {
		_, err = run(ctx, "/usr/bin/systemctl", systemdArgs(plan, "daemon-reload")...)
	}
	return err
}

func report(ctx context.Context, job workerJob, status string) error {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // Task credentials must not be forwarded to environment proxies.
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: job.InsecureTLS}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	reason := ""
	if status == "failed" {
		reason = "local_uninstall_failed"
	}
	data, err := json.Marshal(map[string]string{"status": status, "reason": reason})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, job.Command.CallbackURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+job.Command.CallbackToken)
	req.Header.Set("Content-Type", "application/json")
	rsp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != 200 {
		return fmt.Errorf("uninstall result status %d", rsp.StatusCode)
	}
	return nil
}

// RunWorker is called before normal Edge startup only for the explicit helper
// flag. It never reads a caller-selected executable or shell command from the job.
func RunWorker(ctx context.Context, path string) error {
	if filepath.Base(path) != "job.json" {
		return ErrUnsupported
	}
	dir := filepath.Dir(path)
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 || !ownedFile(info, os.Geteuid(), false) {
		return ErrUnsupported
	}
	info, err = os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || !ownedFile(info, os.Geteuid(), true) || info.Size() > 64*1024 {
		return ErrUnsupported
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var job workerJob
	if err = json.Unmarshal(data, &job); err != nil {
		return err
	}
	if job.Identity.UID != os.Geteuid() || !instancePattern.MatchString(job.Command.TaskID) || job.Command.ExpiresAt <= time.Now().Unix() {
		return ErrUnsupported
	}
	expected, err := os.Executable()
	if err != nil {
		return err
	}
	if expected != filepath.Join(dir, "worker") {
		return ErrUnsupported
	}
	// The job must also bind its callback host. It was already checked against
	// Manager dial addresses by Prepare; the protected job is local, not remote input.
	u, err := url.Parse(job.Command.CallbackURL)
	if err != nil {
		return err
	}
	if err = validateCommand(job.Command, []string{net.JoinHostPort(u.Hostname(), "443")}, time.Now()); err != nil {
		return err
	}
	defer func() {
		for _, file := range []string{path, expected, dir} {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) { /* Bounded leftovers, never broaden cleanup. */
			}
		}
	}()
	// Must have a live receipt before stopping the original service. A Manager
	// outage at this point leaves the service untouched.
	if err = report(ctx, job, "running"); err != nil {
		return err
	}
	result := "completed"
	if err = applyLocked(ctx, job); err != nil {
		result = "failed"
	}
	for attempt := 0; attempt < 4; attempt++ {
		if callbackErr := report(ctx, job, result); callbackErr == nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * time.Duration(attempt+1)):
		}
	}
	return errors.New("uninstall result unconfirmed for task " + strconv.Quote(job.Command.TaskID))
}

func applyLocked(ctx context.Context, job workerJob) (resultErr error) {
	plan, err := Check(ctx, job.Identity, NativeCommand)
	if err != nil {
		return err
	}
	if plan != job.Plan {
		return ErrUnsupported
	}
	lock := filepath.Join(filepath.Dir(plan.Executable), ".lifecycle.lock")
	if err = os.Mkdir(lock, 0700); err != nil {
		return errors.New("another lifecycle operation requires inspection")
	}
	defer func() { resultErr = errors.Join(resultErr, os.Remove(lock)) }()
	return apply(ctx, job, Check, NativeCommand, validateFile, os.Remove)
}

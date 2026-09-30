package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/edge/agent/process"
)

var ErrSessionID = errors.New("invalid Claude session identifier")
var ErrCheckpoint = errors.New("Claude session checkpoint unavailable")

// 暂只支持已验证的默认原生存储位置；长目录名的哈希规则不做猜测。
func transcriptPath(home, project, id string) (string, error) {
	if !filepath.IsAbs(home) || !filepath.IsAbs(project) || !validSessionID(id) {
		return "", ErrCheckpoint
	}
	encoded := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, project)
	if len(encoded) > 200 {
		return "", ErrCheckpoint
	}
	return filepath.Join(home, ".claude", "projects", encoded, id+".jsonl"), nil
}

func checkCheckpoint(path string, resume bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && !resume {
		return nil
	}
	if err != nil || !resume || !info.Mode().IsRegular() || info.Size() == 0 {
		return ErrCheckpoint
	}
	return nil
}

// NewSessionID 在 Edge 生成全新 UUID，不接受用户选择已有的本机会话。
func NewSessionID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(data[:])
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:], nil
}

func validSessionID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return id != "00000000-0000-0000-0000-000000000000"
}

func persistentSpec(i discovery.Installation, project, id string, resume bool) (process.Spec, error) {
	if !validSessionID(id) {
		return process.Spec{}, ErrSessionID
	}
	spec, err := launchSpec(i, project)
	if err != nil {
		return process.Spec{}, err
	}
	args := make([]string, 0, len(spec.Args)+2)
	for _, arg := range spec.Args {
		if arg != "--no-session-persistence" {
			args = append(args, arg)
		}
	}
	flag := "--session-id"
	if resume {
		flag = "--resume"
	}
	spec.Args = append(args, flag, id)
	return spec, nil
}

// StartPersistent 仅供受信会话层使用。新会话 ID 必须由 NewSessionID 生成；
// 恢复 ID 必须来自已验证 owner/access/project/installation 的持久绑定。
// 它不接受会话名称、文件路径或 --continue。工具仍禁用，暂未开放远程入口。
// Close 只结束本驱动创建的进程，不删除 Claude 原生历史。
func StartPersistent(ctx context.Context, i discovery.Installation, project, id string, resume bool) (*Driver, error) {
	return startPersistent(ctx, i, project, id, resume, false)
}

func startPersistent(ctx context.Context, i discovery.Installation, project, id string, resume, interactive bool) (*Driver, error) {
	spec, err := persistentSpec(i, project, id, resume)
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, ErrCheckpoint
	}
	canonical, err := filepath.EvalSymlinks(project)
	if err != nil || canonical != project {
		return nil, ErrCheckpoint
	}
	path, err := transcriptPath(home, project, id)
	if err != nil {
		return nil, err
	}
	if err := checkCheckpoint(path, resume); err != nil {
		return nil, err
	}
	if interactive {
		spec = interactiveSpec(spec)
	}
	return startBoundDriver(ctx, spec, id)
}

// 所有工具遵循原生 default 权限判断及设备配置，不强制询问所有 Bash 调用。
func interactiveSpec(spec process.Spec) process.Spec {
	for i := range spec.Args {
		if spec.Args[i] == "--tools" {
			spec.Args[i+1] = "Read,Glob,Grep,Bash,AskUserQuestion"
		}
	}
	spec.Args = append(spec.Args, "--permission-mode", "default")
	return spec
}

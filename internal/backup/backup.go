// Package backup syncs ~/Library/LaunchAgents plists into a git repository:
// copy plists, write a restore script + disabled-state list, commit, push.
//
// Configuration via env:
//
//	LOCALCTL_BACKUP_DIR     working repo path (default ~/localctl-plist)
//	LOCALCTL_BACKUP_REMOTE  optional push remote URL (empty = local only)
package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"localctl/internal/launchd"
)

// Result reports what one sync did.
type Result struct {
	Copied  int    `json:"copied"`
	Commit  string `json:"commit,omitempty"`
	Pushed  bool   `json:"pushed"`
	Message string `json:"message,omitempty"` // push failure detail etc.
}

// Dir resolves the backup repo path (supports ~).
func Dir() string {
	dir := os.Getenv("LOCALCTL_BACKUP_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "localctl-plist")
	}
	if strings.HasPrefix(dir, "~") {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	return dir
}

// Remote returns the configured push remote (may be empty).
func Remote() string { return os.Getenv("LOCALCTL_BACKUP_REMOTE") }

const restoreScript = `#!/bin/bash
# 在新 Mac 上恢复本仓库中的 LaunchAgents。
# 注意：plist 引用的脚本/程序路径需要先自行恢复，本脚本会报告注册失败的项。
set -u
DIR="$(cd "$(dirname "$0")" && pwd)"
DEST="$HOME/Library/LaunchAgents"
UID_N="$(id -u)"
mkdir -p "$DEST"
ok=0; fail=0
for f in "$DIR"/*.plist; do
  [ -e "$f" ] || continue
  name="$(basename "$f")"
  cp "$f" "$DEST/$name"
  if launchctl bootstrap "gui/$UID_N" "$DEST/$name" 2>/dev/null; then
    echo "已注册: $name"; ok=$((ok+1))
  else
    echo "注册失败（可能已注册或程序路径缺失）: $name"; fail=$((fail+1))
  fi
done
# 恢复禁用状态
if command -v python3 >/dev/null && [ -f "$DIR/disabled.json" ]; then
  python3 - "$DIR/disabled.json" <<'PY'
import json, os, subprocess, sys
uid = os.getuid()
try:
    labels = json.load(open(sys.argv[1]))
except Exception as e:
    sys.exit(0)
for label in labels:
    subprocess.run(["launchctl", "disable", f"gui/{uid}", label])
    print("已恢复禁用状态:", label)
PY
fi
echo "完成：成功 $ok，失败 $fail"
`

func git(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// 服务器/定时任务环境没有 TTY：禁止 SSH 任何交互（口令、host key 确认），
	// 否则 push 会永远挂起；BatchMode 下失败会直接返回错误。
	// SSH_AUTH_SOCK 必须先剔除再按配置追加：execve 环境里重复的 key 只有
	// 第一个生效，launchd 继承的旧 socket 会遮蔽我们指定的那个。
	env := make([]string, 0, len(os.Environ())+2)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "SSH_AUTH_SOCK=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes -o ConnectTimeout=10")
	if sock := os.Getenv("LOCALCTL_SSH_AUTH_SOCK"); sock != "" {
		env = append(env, "SSH_AUTH_SOCK="+sock)
	}
	cmd.Env = env
	// git 会派生 ssh 子进程；超时必须杀整个进程组，否则 ssh 仍握着
	// 输出管道，Wait 会永远阻塞。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		return "", err
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		case <-done:
		}
	}()
	err := cmd.Wait()
	close(done)
	out := strings.TrimSpace(buf.String())
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("git %s 超时（2 分钟）", strings.Join(args, " "))
	}
	return out, err
}

// Sync performs one backup cycle. It is idempotent and safe to run repeatedly.
func Sync() (*Result, error) {
	res := &Result{}
	srcDir, err := launchAgentsDir()
	if err != nil {
		return nil, err
	}
	dir := Dir()
	remote := Remote()

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("读取 LaunchAgents: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	// 初始化仓库与远端（幂等）
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if out, err := git(dir, "init", "-b", "main"); err != nil {
			return nil, fmt.Errorf("git init: %s", out)
		}
	}
	if remote != "" {
		if out, err := git(dir, "remote", "add", "origin", remote); err != nil {
			_, _ = git(dir, "remote", "set-url", "origin", remote)
		} else {
			_ = out
		}
	}
	// commit 需要身份；全局未配置时兜底
	if out, err := git(dir, "config", "user.email"); err != nil || out == "" {
		_, _ = git(dir, "config", "user.email", "localctl@localhost")
		_, _ = git(dir, "config", "user.name", "localctl")
	}

	log.Printf("backup: 同步开始 dir=%s remote=%s", dir, remote)

	// 拷贝 plist
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".plist") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			return nil, err
		}
		res.Copied++
	}

	// 禁用状态清单
	disabled, err := launchd.DisabledMap()
	if err == nil {
		var labels []string
		for label, isDisabled := range disabled {
			if isDisabled {
				labels = append(labels, label)
			}
		}
		sort.Strings(labels)
		data, _ := json.MarshalIndent(labels, "", "  ")
		_ = os.WriteFile(filepath.Join(dir, "disabled.json"), data, 0o644)
	}

	// 恢复脚本
	script := filepath.Join(dir, "restore.sh")
	if err := os.WriteFile(script, []byte(restoreScript), 0o755); err != nil {
		return nil, err
	}

	log.Printf("backup: 拷贝完成 %d 个，开始 git add", res.Copied)
	if _, err := git(dir, "add", "-A"); err != nil {
		return nil, fmt.Errorf("git add: %v", err)
	}
	log.Printf("backup: git add 完成")
	status, _ := git(dir, "status", "--porcelain")
	if status != "" {
		msg := "backup: " + time.Now().Format("2006-01-02 15:04")
		if out, err := git(dir, "commit", "-m", msg); err != nil {
			return nil, fmt.Errorf("git commit: %s", out)
		} else {
			hash, _ := git(dir, "rev-parse", "--short", "HEAD")
			res.Commit = hash
		}
	}

	if remote != "" {
		log.Printf("backup: 开始 push")
		if out, err := git(dir, "push", "-u", "origin", "main"); err != nil {
			res.Pushed = false
			res.Message = "推送到远端失败（本地提交已完成，下次会重试）: " + out
			log.Printf("backup: push 失败: %s", out)
		} else {
			res.Pushed = true
			log.Printf("backup: push 成功")
		}
	} else {
		res.Message = "未配置 LOCALCTL_BACKUP_REMOTE，仅本地提交"
	}
	return res, nil
}

func launchAgentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}

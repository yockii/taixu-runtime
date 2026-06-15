package toolrunner

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	ghttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// ctxTimeout 带超时的 context（git 网络操作用）。
func ctxTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// git.go — 纯 Go（go-git）git 工具：委托交付的产物仓库收发。
// 编译进二进制，docker+裸机+3OS 零外部依赖、免环境检测（同 modernc-sqlite 哲学）。
// 用于 [[project_commission_git_substrate]]：生命 commission.claim 拿到带 token 的 clone URL →
// GitClone 到沙箱 → fs.write 干活 → GitCommitPush 推产物 → commission.deliver 注明 commit。
// 所有操作限沙箱内（checkSandbox 防逃逸），凭据从 URL 解析、不落额外文件。

// gitMaxSeconds 单次 git 网络操作超时（clone/push）。
const gitMaxSeconds = 90 * time.Second

// parseGitCreds 从含凭据的 URL 拆出干净 URL + basic-auth（user:token@host → user,token）。
// 无凭据则 auth=nil。
func parseGitCreds(raw string) (clean string, auth *ghttp.BasicAuth, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", nil, fmt.Errorf("git: 非法 URL: %w", err)
	}
	if u.User != nil {
		user := u.User.Username()
		pass, _ := u.User.Password()
		if user != "" {
			auth = &ghttp.BasicAuth{Username: user, Password: pass}
		}
		u.User = nil
	}
	return u.String(), auth, nil
}

// GitClone 把远端 repo（url 可含 user:token 凭据）clone 到沙箱内 relDir。
// 凭据保留在 origin remote（仅生命自己的 token，沙箱内可读无妨），供后续 push 复用。
func GitClone(cycleID int64, rawURL, relDir string) (Result, error) {
	return audit(cycleID, "git.clone", relDir, func() (string, error) {
		abs, err := checkSandbox(relDir)
		if err != nil {
			return "", err
		}
		clean, auth, err := parseGitCreds(rawURL)
		if err != nil {
			return "", err
		}
		if entries, e := os.ReadDir(abs); e == nil && len(entries) > 0 {
			return "", fmt.Errorf("git.clone: 目标目录 %s 非空（先换个空目录）", relDir)
		}
		ctx, cancel := ctxTimeout(gitMaxSeconds)
		defer cancel()
		// 存回含凭据的 URL 作 origin（push 复用），但用 clean+auth 实际拉取。
		repo, err := gogit.PlainCloneContext(ctx, abs, false, &gogit.CloneOptions{URL: clean, Auth: auth})
		if err != nil {
			return "", fmt.Errorf("git.clone 失败: %w", err)
		}
		if auth != nil {
			_ = repo.DeleteRemote("origin")
			_, _ = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{rawURL}})
		}
		head, _ := repo.Head()
		ref := ""
		if head != nil {
			ref = head.Hash().String()[:8]
		}
		return fmt.Sprintf("cloned to %s (HEAD %s)", relDir, ref), nil
	})
}

// GitCommitPush 暂存 relDir 下全部改动 → commit（authorName 署名）→ push 到 origin 当前分支。
// 返回新 commit 全 hash（生命写进 commission.deliver 作交付凭据）。无改动则跳过 commit、仍尝试 push。
func GitCommitPush(cycleID int64, relDir, message, authorName, authorEmail string) (Result, error) {
	return audit(cycleID, "git.commit_push", relDir, func() (string, error) {
		abs, err := checkSandbox(relDir)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(message) == "" {
			message = "交付更新"
		}
		if authorName == "" {
			authorName = "数字生命"
		}
		if authorEmail == "" {
			authorEmail = "life@taixu.icu"
		}
		repo, err := gogit.PlainOpen(abs)
		if err != nil {
			return "", fmt.Errorf("git.commit_push: 打不开 repo（先 git.clone）: %w", err)
		}
		wt, err := repo.Worktree()
		if err != nil {
			return "", err
		}
		if err := wt.AddGlob("."); err != nil {
			return "", fmt.Errorf("git add 失败: %w", err)
		}
		st, err := wt.Status()
		if err != nil {
			return "", err
		}
		var hash string
		if !st.IsClean() {
			when := time.Now()
			h, err := wt.Commit(message, &gogit.CommitOptions{
				Author: &object.Signature{Name: authorName, Email: authorEmail, When: when},
			})
			if err != nil {
				return "", fmt.Errorf("git commit 失败: %w", err)
			}
			hash = h.String()
		} else {
			if head, e := repo.Head(); e == nil {
				hash = head.Hash().String()
			}
		}
		// push：从 origin remote URL 解析凭据。
		auth, err := originAuth(repo)
		if err != nil {
			return "", err
		}
		ctx, cancel := ctxTimeout(gitMaxSeconds)
		defer cancel()
		err = repo.PushContext(ctx, &gogit.PushOptions{RemoteName: "origin", Auth: auth})
		if err != nil && err != gogit.NoErrAlreadyUpToDate {
			return "", fmt.Errorf("git push 失败: %w", err)
		}
		return fmt.Sprintf("committed+pushed %s", hash), nil
	})
}

// originAuth 从 repo 的 origin remote URL 解析 basic-auth（push 复用 clone 时的 token）。
func originAuth(repo *gogit.Repository) (*ghttp.BasicAuth, error) {
	rem, err := repo.Remote("origin")
	if err != nil {
		return nil, fmt.Errorf("git: 无 origin remote: %w", err)
	}
	urls := rem.Config().URLs
	if len(urls) == 0 {
		return nil, nil
	}
	_, auth, err := parseGitCreds(urls[0])
	return auth, err
}

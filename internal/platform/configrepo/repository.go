package configrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	ssh2 "golang.org/x/crypto/ssh"
)

var ErrSourcePathNotFound = errors.New("config repo source path not found")
var ErrRepositorySyncFailed = errors.New("config repo sync failed")

var DefaultRepository *Repository

const (
	FixedRepositoryURL = "ssh://git@github.com/bsonger/devflow-config-repo.git"
	FixedBranch        = "main"
	DefaultSSHKeyPath  = "/etc/devflow/git-ssh/id_rsa"
)

type Options struct {
	RootDir    string
	DefaultRef string
	SSHKeyPath string
}

type Snapshot struct {
	SourcePath   string
	SourceCommit string
	SourceDigest string
	Files        []File
}

type File struct {
	Name    string
	Content string
}

type gitSyncer interface {
	Sync(ctx context.Context, rootDir, ref string) (string, error)
}

type Repository struct {
	rootDir    string
	defaultRef string
	sshKeyPath string
	syncer     gitSyncer
}

func NewRepository(opts Options) *Repository {
	return &Repository{
		rootDir:    opts.RootDir,
		defaultRef: opts.DefaultRef,
		sshKeyPath: opts.SSHKeyPath,
		syncer:     commandGitSyncer{sshKeyPath: opts.SSHKeyPath},
	}
}

func (r *Repository) ReadSnapshot(ctx context.Context, sourcePath, env string) (*Snapshot, error) {
	sourceCommit := r.defaultRefOrMain()
	if commit, err := r.sync(ctx); err != nil {
		return nil, err
	} else if commit != "" {
		sourceCommit = commit
	}
	resolved, err := resolveLayout(r.rootDir, sourcePath, env)
	if err != nil {
		return nil, err
	}

	files := make([]File, 0, len(resolved.Entries))
	hash := sha256.New()
	for _, entry := range resolved.Entries {
		content, err := os.ReadFile(entry.DiskPath)
		if err != nil {
			return nil, err
		}
		files = append(files, File{
			Name:    entry.Name,
			Content: string(content),
		})
		hash.Write([]byte(entry.Name))
		hash.Write([]byte{'\n'})
		hash.Write(content)
		hash.Write([]byte{'\n'})
	}

	return &Snapshot{
		SourcePath:   strings.TrimPrefix(filepath.ToSlash(resolved.SourcePath), "./"),
		SourceCommit: sourceCommit,
		SourceDigest: hex.EncodeToString(hash.Sum(nil)),
		Files:        files,
	}, nil
}

func (r *Repository) sync(ctx context.Context) (string, error) {
	if r == nil || r.rootDir == "" || r.syncer == nil {
		return "", nil
	}
	gitDir := filepath.Join(r.rootDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		if os.IsNotExist(err) {
			if entries, readErr := os.ReadDir(r.rootDir); readErr == nil && len(entries) > 0 {
				return "", nil
			}
			if cloneErr := ensureClonedRepository(ctx, r.rootDir, r.defaultRefOrMain(), r.sshKeyPath); cloneErr != nil {
				return "", fmt.Errorf("%w: %v", ErrRepositorySyncFailed, cloneErr)
			}
		} else {
			return "", err
		}
	}
	commit, err := r.syncer.Sync(ctx, r.rootDir, r.defaultRefOrMain())
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrRepositorySyncFailed, err)
	}
	return commit, nil
}

func (r *Repository) defaultRefOrMain() string {
	if r == nil || r.defaultRef == "" {
		return FixedBranch
	}
	return r.defaultRef
}

type commandGitSyncer struct {
	sshKeyPath string
}

func ensureClonedRepository(ctx context.Context, rootDir, ref, sshKeyPath string) error {
	auth, err := defaultAuthMethod(sshKeyPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(rootDir), 0o755); err != nil {
		return err
	}
	_, err = git.PlainCloneContext(ctx, rootDir, false, &git.CloneOptions{
		URL:           FixedRepositoryURL,
		ReferenceName: plumbing.NewBranchReferenceName(ref),
		SingleBranch:  true,
		Depth:         1,
		Progress:      nil,
		Auth:          auth,
	})
	if err != nil {
		return fmt.Errorf("git clone %s %s: %w", FixedRepositoryURL, rootDir, err)
	}
	return nil
}

func (s commandGitSyncer) Sync(ctx context.Context, rootDir, ref string) (string, error) {
	auth, err := defaultAuthMethod(s.sshKeyPath)
	if err != nil {
		return "", err
	}
	repo, err := git.PlainOpen(rootDir)
	if err != nil {
		return "", fmt.Errorf("git open %s: %w", rootDir, err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("git worktree %s: %w", rootDir, err)
	}
	if err := worktree.PullContext(ctx, &git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(ref),
		SingleBranch:  true,
		Depth:         1,
		Progress:      nil,
		Force:         false,
		Auth:          auth,
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "", fmt.Errorf("git pull origin %s: %w", ref, err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("git head %s: %w", rootDir, err)
	}
	return head.Hash().String(), nil
}

func defaultAuthMethod(path string) (*gitssh.PublicKeys, error) {
	keyPath := strings.TrimSpace(path)
	if keyPath == "" {
		keyPath = DefaultSSHKeyPath
	}
	if _, err := os.Stat(keyPath); err != nil {
		return nil, fmt.Errorf("git ssh key %s: %w", keyPath, err)
	}
	auth, err := gitssh.NewPublicKeysFromFile("git", keyPath, "")
	if err != nil {
		return nil, fmt.Errorf("git ssh key %s: %w", keyPath, err)
	}
	auth.HostKeyCallbackHelper = gitssh.HostKeyCallbackHelper{
		HostKeyCallback: ssh2.InsecureIgnoreHostKey(),
	}
	return auth, nil
}

package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitManager struct {
	StoragePath string
}

func NewGitManager(storagePath string) *GitManager {
	if err := os.MkdirAll(storagePath, 0750); err != nil {
		slog.Error(fmt.Sprintf("Cannot create git storage: %v", err))
		os.Exit(1)
	}
	return &GitManager{StoragePath: storagePath}
}

type CloneResult struct {
	LocalPath string
	Tags      []string
	CommitRef string
	Error     error
}

func (gm *GitManager) CloneRepository(ctx context.Context, repoURL, pluginID string) (*CloneResult, error) {
	destPath := filepath.Join(gm.StoragePath, pluginID)

	if err := os.RemoveAll(destPath); err != nil {
		return nil, fmt.Errorf("cleanup failed: %w", err)
	}

	cloneOptions := &git.CloneOptions{
		URL:          repoURL,
		Progress:     os.Stdout,
		Tags:         git.AllTags,
		Depth:        0,
		SingleBranch: false,
	}

	repo, err := git.PlainCloneContext(ctx, destPath, false, cloneOptions)
	if err != nil {
		return nil, fmt.Errorf("clone failed for %s: %w", repoURL, err)
	}

	tags, err := gm.listTags(repo)
	if err != nil {
		tags = []string{}
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("cannot get HEAD: %w", err)
	}

	return &CloneResult{
		LocalPath: destPath,
		Tags:      tags,
		CommitRef: head.Hash().String(),
	}, nil
}

func (gm *GitManager) FetchUpdates(ctx context.Context, pluginID string) error {
	repoPath := filepath.Join(gm.StoragePath, pluginID)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("cannot open repo: %w", err)
	}
	fetchOptions := &git.FetchOptions{Tags: git.AllTags, Force: true}
	if err := repo.FetchContext(ctx, fetchOptions); err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("fetch failed: %w", err)
	}
	return nil
}

func (gm *GitManager) GenerateDiff(pluginID, fromRef, toRef string) (string, error) {
	repoPath := filepath.Join(gm.StoragePath, pluginID)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("cannot open repo: %w", err)
	}

	fromCommit, err := gm.resolveRefToCommit(repo, fromRef)
	if err != nil {
		return "", fmt.Errorf("invalid 'from' ref (%s): %w", fromRef, err)
	}
	toCommit, err := gm.resolveRefToCommit(repo, toRef)
	if err != nil {
		return "", fmt.Errorf("invalid 'to' ref (%s): %w", toRef, err)
	}

	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", err
	}
	toTree, err := toCommit.Tree()
	if err != nil {
		return "", err
	}

	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return "", err
	}

	patch, err := changes.Patch()
	if err != nil {
		return "", err
	}
	return patch.String(), nil
}

// FileTreeEntry represents a node in the file tree
type FileTreeEntry struct {
	Name     string          `json:"name"`
	Path     string          `json:"path"`
	IsDir    bool            `json:"is_dir"`
	Size     int64           `json:"size"`
	Children []FileTreeEntry `json:"children,omitempty"`
}

// GetFileTree returns the full file tree at a given ref
func (gm *GitManager) GetFileTree(pluginID, ref string) ([]FileTreeEntry, error) {
	repoPath := filepath.Join(gm.StoragePath, pluginID)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	commit, err := gm.resolveRefToCommit(repo, ref)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	root := make(map[string]*FileTreeEntry)
	var topLevel []FileTreeEntry

	tree.Files().ForEach(func(f *object.File) error {
		parts := strings.Split(f.Name, "/")
		if len(parts) == 1 {
			topLevel = append(topLevel, FileTreeEntry{
				Name:  f.Name,
				Path:  f.Name,
				IsDir: false,
				Size:  f.Size,
			})
		} else {
			gm.insertIntoTree(root, &topLevel, parts, f.Size)
		}
		return nil
	})

	sortTree(topLevel)
	return topLevel, nil
}

func (gm *GitManager) insertIntoTree(dirs map[string]*FileTreeEntry, topLevel *[]FileTreeEntry, parts []string, size int64) {
	currentPath := ""
	for i, part := range parts {
		if i > 0 {
			currentPath += "/"
		}
		currentPath += part

		isLast := i == len(parts)-1

		if isLast {
			entry := FileTreeEntry{Name: part, Path: currentPath, IsDir: false, Size: size}
			if i == 0 {
				*topLevel = append(*topLevel, entry)
			} else {
				parentPath := currentPath[:len(currentPath)-len(part)-1]
				if parent, ok := dirs[parentPath]; ok {
					parent.Children = append(parent.Children, entry)
				}
			}
		} else {
			if _, exists := dirs[currentPath]; !exists {
				dir := &FileTreeEntry{Name: part, Path: currentPath, IsDir: true, Children: []FileTreeEntry{}}
				dirs[currentPath] = dir
				if i == 0 {
					*topLevel = append(*topLevel, *dir)
					dirs[currentPath] = &(*topLevel)[len(*topLevel)-1]
				} else {
					parentPath := currentPath[:len(currentPath)-len(part)-1]
					if parent, ok := dirs[parentPath]; ok {
						parent.Children = append(parent.Children, *dir)
						dirs[currentPath] = &parent.Children[len(parent.Children)-1]
					}
				}
			}
		}
	}
}

func sortTree(entries []FileTreeEntry) {
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			// Directories first, then alphabetical
			if (!entries[i].IsDir && entries[j].IsDir) ||
				(entries[i].IsDir == entries[j].IsDir && entries[i].Name > entries[j].Name) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
	for idx := range entries {
		if entries[idx].IsDir && len(entries[idx].Children) > 0 {
			sortTree(entries[idx].Children)
		}
	}
}

// GetFileContent returns the content of a single file at a given ref
func (gm *GitManager) GetFileContent(pluginID, ref, filePath string) (string, error) {
	repoPath := filepath.Join(gm.StoragePath, pluginID)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", err
	}

	commit, err := gm.resolveRefToCommit(repo, ref)
	if err != nil {
		return "", err
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", err
	}

	file, err := tree.File(filePath)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", filePath)
	}

	reader, err := file.Reader()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (gm *GitManager) GetRepoPathForPlugin(pluginID string) string {
	return filepath.Join(gm.StoragePath, pluginID)
}

func (gm *GitManager) CheckoutTag(pluginID, tag string) error {
	repoPath := filepath.Join(gm.StoragePath, pluginID)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}
	commit, err := gm.resolveRefToCommit(repo, tag)
	if err != nil {
		return err
	}
	return worktree.Checkout(&git.CheckoutOptions{Hash: commit.Hash, Force: true})
}

func (gm *GitManager) ExtractChangelog(pluginID, tag string) (string, error) {
	repoPath := filepath.Join(gm.StoragePath, pluginID)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", err
	}
	commit, err := gm.resolveRefToCommit(repo, tag)
	if err != nil {
		return "", err
	}
	return commit.Message, nil
}

func (gm *GitManager) GetTags(pluginID string) ([]string, error) {
	repoPath := filepath.Join(gm.StoragePath, pluginID)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}
	return gm.listTags(repo)
}

func (gm *GitManager) listTags(repo *git.Repository) ([]string, error) {
	tagRefs, err := repo.Tags()
	if err != nil {
		return nil, err
	}

	type tagEntry struct {
		name string
		when time.Time
	}
	var entries []tagEntry

	tagRefs.ForEach(func(ref *plumbing.Reference) error {
		name := strings.TrimPrefix(ref.Name().String(), "refs/tags/")
		tagObj, err := repo.TagObject(ref.Hash())
		if err == nil {
			entries = append(entries, tagEntry{name: name, when: tagObj.Tagger.When})
		} else {
			commit, err := repo.CommitObject(ref.Hash())
			if err == nil {
				entries = append(entries, tagEntry{name: name, when: commit.Author.When})
			}
		}
		return nil
	})

	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].when.Before(entries[j].when) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	tags := make([]string, len(entries))
	for i, e := range entries {
		tags[i] = e.name
	}
	return tags, nil
}

func (gm *GitManager) resolveRefToCommit(repo *git.Repository, ref string) (*object.Commit, error) {
	tagRef, err := repo.Tag(ref)
	if err == nil {
		tagObj, err := repo.TagObject(tagRef.Hash())
		if err == nil {
			commit, err := repo.CommitObject(tagObj.Target)
			if err == nil {
				return commit, nil
			}
		}
		commit, err := repo.CommitObject(tagRef.Hash())
		if err == nil {
			return commit, nil
		}
	}

	hash := plumbing.NewHash(ref)
	commit, err := repo.CommitObject(hash)
	if err == nil {
		return commit, nil
	}

	branchRef, err := repo.Reference(plumbing.NewBranchReferenceName(ref), true)
	if err == nil {
		return repo.CommitObject(branchRef.Hash())
	}

	// Try HEAD
	if ref == "HEAD" || ref == "" {
		headRef, err := repo.Head()
		if err == nil {
			return repo.CommitObject(headRef.Hash())
		}
	}

	return nil, fmt.Errorf("cannot resolve ref '%s'", ref)
}

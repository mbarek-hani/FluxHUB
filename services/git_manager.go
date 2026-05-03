package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitManager struct {
	StoragePath string // Chemin de base pour les clones
}

func NewGitManager(storagePath string) *GitManager {
	if err := os.MkdirAll(storagePath, 0750); err != nil {
		log.Fatalf("Impossible de créer le dossier de stockage Git: %v", err)
	}
	return &GitManager{StoragePath: storagePath}
}

// CloneResult contient les résultats d'un clonage
type CloneResult struct {
	LocalPath string
	Tags      []string
	CommitRef string
	Error     error
}

// CloneRepository clone un dépôt Git dans un dossier unique
// Retourne le chemin local et les tags disponibles
func (gm *GitManager) CloneRepository(ctx context.Context, repoURL, pluginID string) (*CloneResult, error) {
	// Chemin de destination unique basé sur l'ID du plugin
	destPath := filepath.Join(gm.StoragePath, pluginID)

	// Supprimer un éventuel clone précédent pour repartir proprement
	if err := os.RemoveAll(destPath); err != nil {
		return nil, fmt.Errorf("impossible de nettoyer le dossier précédent: %w", err)
	}

	log.Printf("Clonage de %s vers %s", repoURL, destPath)

	// Options de clonage avec timeout via context
	cloneOptions := &git.CloneOptions{
		URL:          repoURL,
		Progress:     os.Stdout,
		Tags:         git.AllTags,
		Depth:        0,
		SingleBranch: false,
	}

	repo, err := git.PlainCloneContext(ctx, destPath, false, cloneOptions)
	if err != nil {
		return nil, fmt.Errorf("échec du clonage de %s: %w", repoURL, err)
	}

	// Récupérer la liste des tags
	tags, err := gm.listTags(repo)
	if err != nil {
		log.Printf("Impossible de lister les tags: %v", err)
		tags = []string{}
	}

	// Récupérer le commit HEAD courant
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("impossible de récupérer HEAD: %w", err)
	}

	log.Printf("Clonage terminé. Tags trouvés: %v", tags)

	return &CloneResult{
		LocalPath: destPath,
		Tags:      tags,
		CommitRef: head.Hash().String(),
	}, nil
}

// FetchUpdates met à jour un dépôt déjà cloné
func (gm *GitManager) FetchUpdates(ctx context.Context, pluginID string) error {
	repoPath := filepath.Join(gm.StoragePath, pluginID)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("impossible d'ouvrir le dépôt local: %w", err)
	}

	fetchOptions := &git.FetchOptions{
		Tags:  git.AllTags,
		Force: true,
	}

	if err := repo.FetchContext(ctx, fetchOptions); err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("échec du fetch: %w", err)
	}

	return nil
}

func (gm *GitManager) GenerateDiff(pluginID, fromRef, toRef string) (string, error) {
	repoPath := filepath.Join(gm.StoragePath, pluginID)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("impossible d'ouvrir le dépôt: %w", err)
	}

	// Résoudre fromRef en commit
	fromCommit, err := gm.resolveRefToCommit(repo, fromRef)
	if err != nil {
		return "", fmt.Errorf("référence 'from' invalide (%s): %w", fromRef, err)
	}

	// Résoudre toRef en commit
	toCommit, err := gm.resolveRefToCommit(repo, toRef)
	if err != nil {
		return "", fmt.Errorf("référence 'to' invalide (%s): %w", toRef, err)
	}

	// Obtenir les arbres de fichiers pour chaque commit
	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("impossible d'obtenir l'arbre 'from': %w", err)
	}

	toTree, err := toCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("impossible d'obtenir l'arbre 'to': %w", err)
	}

	// Générer le patch entre les deux arbres
	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return "", fmt.Errorf("impossible de calculer le diff: %w", err)
	}

	patch, err := changes.Patch()
	if err != nil {
		return "", fmt.Errorf("impossible de générer le patch: %w", err)
	}

	return patch.String(), nil
}

// GetFilesAtTag retourne la liste des fichiers à un tag donné
func (gm *GitManager) GetFilesAtTag(pluginID, tag string) ([]string, error) {
	repoPath := filepath.Join(gm.StoragePath, pluginID)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("impossible d'ouvrir le dépôt: %w", err)
	}

	commit, err := gm.resolveRefToCommit(repo, tag)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	var files []string
	tree.Files().ForEach(func(f *object.File) error {
		files = append(files, f.Name)
		return nil
	})

	return files, nil
}

// GetRepoPathForPlugin retourne le chemin local d'un dépôt cloné
func (gm *GitManager) GetRepoPathForPlugin(pluginID string) string {
	return filepath.Join(gm.StoragePath, pluginID)
}

// CheckoutTag checkout un tag spécifique dans le working tree
func (gm *GitManager) CheckoutTag(pluginID, tag string) error {
	repoPath := filepath.Join(gm.StoragePath, pluginID)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("impossible d'ouvrir le dépôt: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("impossible d'obtenir le worktree: %w", err)
	}

	// Résoudre le hash du tag
	commit, err := gm.resolveRefToCommit(repo, tag)
	if err != nil {
		return err
	}

	return worktree.Checkout(&git.CheckoutOptions{
		Hash:  commit.Hash,
		Force: true,
	})
}

// ExtractChangelog extrait le message du commit associé à un tag
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

// Méthodes privées

// listTags retourne tous les tags du dépôt triés par date
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

		// Essayer de résoudre comme tag annoté d'abord
		tagObj, err := repo.TagObject(ref.Hash())
		if err == nil {
			entries = append(entries, tagEntry{name: name, when: tagObj.Tagger.When})
		} else {
			// Tag léger (lightweight tag) - pointer directement sur le commit
			commit, err := repo.CommitObject(ref.Hash())
			if err == nil {
				entries = append(entries, tagEntry{name: name, when: commit.Author.When})
			}
		}
		return nil
	})

	// Trier par date décroissante
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

// resolveRefToCommit résout une référence (tag, branche, hash) en objet commit
func (gm *GitManager) resolveRefToCommit(repo *git.Repository, ref string) (*object.Commit, error) {
	// Essayer comme tag annoté
	tagRef, err := repo.Tag(ref)
	if err == nil {
		tagObj, err := repo.TagObject(tagRef.Hash())
		if err == nil {
			// Tag annoté -> pointer vers le commit
			commit, err := repo.CommitObject(tagObj.Target)
			if err == nil {
				return commit, nil
			}
		}
		// Tag léger -> le hash pointe directement sur le commit
		commit, err := repo.CommitObject(tagRef.Hash())
		if err == nil {
			return commit, nil
		}
	}

	// Essayer comme hash de commit direct
	hash := plumbing.NewHash(ref)
	commit, err := repo.CommitObject(hash)
	if err == nil {
		return commit, nil
	}

	// Essayer comme branche
	branchRef, err := repo.Reference(
		plumbing.NewBranchReferenceName(ref),
		true,
	)
	if err == nil {
		return repo.CommitObject(branchRef.Hash())
	}

	return nil, fmt.Errorf("impossible de résoudre la référence '%s'", ref)
}

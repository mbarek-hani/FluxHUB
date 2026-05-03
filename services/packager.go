package services

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Packager gère la création d'archives ZIP des plugins
type Packager struct {
	OutputPath string // Dossier de sortie pour les ZIPs
}

// NewPackager crée une nouvelle instance de Packager
func NewPackager(outputPath string) *Packager {
	if err := os.MkdirAll(outputPath, 0750); err != nil {
		panic(fmt.Sprintf("Impossible de créer le dossier de sortie ZIP: %v", err))
	}
	return &Packager{OutputPath: outputPath}
}

// PackagePlugin crée un fichier ZIP du plugin (sans le dossier .git)
// Retourne le chemin du fichier ZIP créé
func (p *Packager) PackagePlugin(sourcePath, pluginID, version string) (string, error) {
	// Nom du fichier ZIP : pluginID-version.zip
	zipFileName := fmt.Sprintf("%s-%s.zip", pluginID, version)
	zipFilePath := filepath.Join(p.OutputPath, zipFileName)

	// Vérifier que le sourcePath existe
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return "", fmt.Errorf("le chemin source n'existe pas: %s", sourcePath)
	}

	// Créer le fichier ZIP
	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return "", fmt.Errorf("impossible de créer le fichier ZIP: %w", err)
	}
	defer zipFile.Close()

	// Créer le writer ZIP
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walker le répertoire source
	err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculer le chemin relatif
		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("impossible de calculer le chemin relatif: %w", err)
		}

		// Normaliser les séparateurs de chemin (Windows → Unix)
		relPath = filepath.ToSlash(relPath)

		// ---- Exclusions ----
		// Exclure le dossier .git et son contenu
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Exclure les fichiers cachés .git* au niveau racine
		if strings.HasPrefix(info.Name(), ".git") {
			return nil
		}

		// Exclure node_modules
		if info.IsDir() && info.Name() == "node_modules" {
			return filepath.SkipDir
		}

		// Exclure les fichiers de test (optionnel)
		if info.IsDir() && info.Name() == "tests" {
			return filepath.SkipDir
		}

		// Ignorer la racine elle-même
		if relPath == "." {
			return nil
		}

		// Créer l'en-tête ZIP
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("impossible de créer l'en-tête ZIP pour %s: %w", relPath, err)
		}

		// Préfixer les entrées avec le nom du plugin pour une extraction propre
		// Ex: ai-analytics-1.0.0/src/Plugin.php
		header.Name = fmt.Sprintf("%s-%s/%s", pluginID, version, relPath)

		if info.IsDir() {
			header.Name += "/"
		} else {
			// Utiliser la compression Deflate pour les fichiers
			header.Method = zip.Deflate
		}

		// Écrire l'entrée dans le ZIP
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("impossible de créer l'entrée ZIP pour %s: %w", relPath, err)
		}

		// Copier le contenu du fichier (pas pour les dossiers)
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("impossible d'ouvrir %s: %w", path, err)
			}
			defer file.Close()

			if _, err := io.Copy(writer, file); err != nil {
				return fmt.Errorf("impossible de copier %s dans le ZIP: %w", relPath, err)
			}
		}

		return nil
	})

	if err != nil {
		// Supprimer le ZIP partiel en cas d'erreur
		os.Remove(zipFilePath)
		return "", fmt.Errorf("erreur lors du packaging: %w", err)
	}

	return zipFilePath, nil
}

// GetZipPath retourne le chemin d'un ZIP existant
func (p *Packager) GetZipPath(pluginID, version string) string {
	zipFileName := fmt.Sprintf("%s-%s.zip", pluginID, version)
	return filepath.Join(p.OutputPath, zipFileName)
}

// ZipExists vérifie si un ZIP existe déjà pour ce plugin/version
func (p *Packager) ZipExists(pluginID, version string) bool {
	_, err := os.Stat(p.GetZipPath(pluginID, version))
	return !os.IsNotExist(err)
}

// CleanupOldVersions supprime les anciens ZIPs (optionnel, pour économiser de l'espace)
func (p *Packager) CleanupOldVersions(pluginID string, keepVersions []string) error {
	pattern := filepath.Join(p.OutputPath, fmt.Sprintf("%s-*.zip", pluginID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	keepSet := make(map[string]bool)
	for _, v := range keepVersions {
		keepSet[fmt.Sprintf("%s-%s.zip", pluginID, v)] = true
	}

	for _, match := range matches {
		if !keepSet[filepath.Base(match)] {
			if err := os.Remove(match); err != nil {
				fmt.Printf("Impossible de supprimer %s: %v\n", match, err)
			}
		}
	}

	return nil
}

package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbarek-hani/FluxHUB/models"
)

// DangerousPattern: représente un pattern de code dangereux à détecter
type DangerousPattern struct {
	Pattern  string
	Severity string // critical, warning, info
	Reason   string
}

// dangerousPatterns est la liste des patterns PHP dangereux à détecter
var dangerousPatterns = []DangerousPattern{
	// Exécution de commandes système - CRITIQUE
	{Pattern: "exec(", Severity: "critical", Reason: "Exécution de commande système"},
	{Pattern: "shell_exec(", Severity: "critical", Reason: "Exécution de commande shell"},
	{Pattern: "system(", Severity: "critical", Reason: "Exécution de commande système"},
	{Pattern: "passthru(", Severity: "critical", Reason: "Exécution de commande avec output"},
	{Pattern: "popen(", Severity: "critical", Reason: "Ouverture d'un processus"},
	{Pattern: "proc_open(", Severity: "critical", Reason: "Ouverture d'un processus avancé"},
	{Pattern: "`", Severity: "critical", Reason: "Backtick = exécution shell"},

	// Exécution de code dynamique - CRITIQUE
	{Pattern: "eval(", Severity: "critical", Reason: "Exécution de code PHP dynamique"},
	{Pattern: "assert(", Severity: "warning", Reason: "assert() peut exécuter du code PHP"},
	{Pattern: "preg_replace_callback(", Severity: "warning", Reason: "Callback potentiellement dangereux"},
	{Pattern: "create_function(", Severity: "critical", Reason: "Création de fonction dynamique (deprecated)"},
	{Pattern: "call_user_func(", Severity: "warning", Reason: "Appel de fonction dynamique"},
	{Pattern: "call_user_func_array(", Severity: "warning", Reason: "Appel de tableau de fonctions dynamique"},

	// Requêtes SQL brutes - AVERTISSEMENT
	{Pattern: "DB::raw(", Severity: "warning", Reason: "Requête SQL brute potentiellement non sécurisée"},
	{Pattern: "DB::select(", Severity: "info", Reason: "Requête SQL directe - vérifier les injections"},
	{Pattern: "DB::statement(", Severity: "warning", Reason: "Déclaration SQL brute"},
	{Pattern: "DB::unprepared(", Severity: "critical", Reason: "Requête SQL non préparée"},
	{Pattern: "whereRaw(", Severity: "warning", Reason: "Clause WHERE brute - risque d'injection SQL"},
	{Pattern: "orderByRaw(", Severity: "warning", Reason: "ORDER BY brut - risque d'injection SQL"},
	{Pattern: "selectRaw(", Severity: "warning", Reason: "SELECT brut - risque d'injection SQL"},

	// Manipulation de fichiers sensibles - AVERTISSEMENT
	{Pattern: "file_get_contents(", Severity: "info", Reason: "Lecture de fichier - vérifier le path"},
	{Pattern: "file_put_contents(", Severity: "warning", Reason: "Écriture de fichier - risque d'écriture arbitraire"},
	{Pattern: "fopen(", Severity: "info", Reason: "Ouverture de fichier - vérifier le path"},
	{Pattern: "unlink(", Severity: "warning", Reason: "Suppression de fichier - risque de suppression arbitraire"},
	{Pattern: "rename(", Severity: "info", Reason: "Renommage de fichier - vérifier les paths"},

	// Sérialisation - CRITIQUE
	{Pattern: "unserialize(", Severity: "critical", Reason: "Désérialisation - risque RCE via objets PHP"},
	{Pattern: "serialize(", Severity: "info", Reason: "Sérialisation - vérifier les données"},

	// Réseau - AVERTISSEMENT
	{Pattern: "curl_exec(", Severity: "warning", Reason: "Requête réseau sortante - vérifier les URLs"},
	{Pattern: "fsockopen(", Severity: "warning", Reason: "Ouverture de socket réseau"},

	// Exfiltration potentielle
	{Pattern: "base64_decode(", Severity: "warning", Reason: "Décodage base64 - potentielle obfuscation"},
	{Pattern: "gzinflate(", Severity: "warning", Reason: "Décompression - potentielle obfuscation"},
	{Pattern: "str_rot13(", Severity: "warning", Reason: "ROT13 - technique d'obfuscation"},
}

// CodeScanner effectue l'analyse statique du code PHP
type CodeScanner struct{}

func NewCodeScanner() *CodeScanner {
	return &CodeScanner{}
}

// ScanDirectory analyse récursivement un répertoire PHP
func (s *CodeScanner) ScanDirectory(rootPath string) (*models.ScanReport, error) {
	report := &models.ScanReport{
		HasDangerousCode: false,
		Findings:         []models.ScanFinding{},
		ScannedFiles:     0,
		TotalIssues:      0,
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignorer le dossier .git
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Analyser uniquement les fichiers PHP
		if !info.IsDir() && filepath.Ext(path) == ".php" {
			findings, err := s.scanFile(path, rootPath)
			if err != nil {
				// Logger l'erreur mais continuer l'analyse
				fmt.Printf("Impossible d'analyser %s: %v\n", path, err)
				return nil
			}
			report.ScannedFiles++
			report.Findings = append(report.Findings, findings...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("erreur lors du walk du répertoire: %w", err)
	}

	report.TotalIssues = len(report.Findings)
	report.HasDangerousCode = s.hasCriticalFindings(report.Findings)

	return report, nil
}

// ScanReportToJSON convertit un rapport en JSON
func (s *CodeScanner) ScanReportToJSON(report *models.ScanReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// hasCriticalFindings vérifie s'il y a des findings critiques
func (s *CodeScanner) hasCriticalFindings(findings []models.ScanFinding) bool {
	for _, f := range findings {
		if f.Severity == "critical" {
			return true
		}
	}
	return false
}

// scanFile analyse un fichier PHP ligne par ligne
func (s *CodeScanner) scanFile(filePath, rootPath string) ([]models.ScanFinding, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var findings []models.ScanFinding
	lineNum := 0
	scanner := bufio.NewScanner(file)

	// Augmenter la taille du buffer pour les longues lignes
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 64*1024)

	// Chemin relatif pour l'affichage
	relPath, _ := filepath.Rel(rootPath, filePath)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		// Ignorer les commentaires (simpliste mais efficace)
		if strings.HasPrefix(trimmedLine, "//") ||
			strings.HasPrefix(trimmedLine, "#") ||
			strings.HasPrefix(trimmedLine, "*") ||
			strings.HasPrefix(trimmedLine, "/*") {
			continue
		}

		// Vérifier chaque pattern dangereux
		for _, pattern := range dangerousPatterns {
			if strings.Contains(line, pattern.Pattern) {
				// Créer un extrait de contexte (tronqué à 150 chars)
				context := strings.TrimSpace(line)
				if len(context) > 150 {
					context = context[:150] + "..."
				}

				findings = append(findings, models.ScanFinding{
					File:     relPath,
					Line:     lineNum,
					Pattern:  pattern.Pattern,
					Severity: pattern.Severity,
					Context:  context,
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return findings, fmt.Errorf("erreur de lecture de %s: %w", filePath, err)
	}

	return findings, nil
}

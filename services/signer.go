package services

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

// Signer gère la signature numérique RSA des fichiers
type Signer struct {
	privateKey *rsa.PrivateKey
}

// NewSigner charge la clé privée RSA depuis un fichier PEM
func NewSigner(privateKeyPath string) (*Signer, error) {
	// Lire le fichier PEM
	pemData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("impossible de lire la clé privée (%s): %w", privateKeyPath, err)
	}

	// Décoder le bloc PEM
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("format PEM invalide dans %s", privateKeyPath)
	}

	var privateKey *rsa.PrivateKey

	// Supporter PKCS1 et PKCS8
	switch block.Type {
	case "RSA PRIVATE KEY":
		// Format PKCS1
		privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("impossible de parser la clé PKCS1: %w", err)
		}

	case "PRIVATE KEY":
		// Format PKCS8
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("impossible de parser la clé PKCS8: %w", err)
		}
		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("la clé PKCS8 n'est pas de type RSA")
		}

	default:
		return nil, fmt.Errorf("type de clé PEM non supporté: %s", block.Type)
	}

	// Valider la clé
	if err := privateKey.Validate(); err != nil {
		return nil, fmt.Errorf("clé RSA invalide: %w", err)
	}

	return &Signer{privateKey: privateKey}, nil
}

// GenerateKeyPair génère une paire de clés RSA et les sauvegarde
// Utile pour l'initialisation du système
func GenerateKeyPair(privateKeyPath, publicKeyPath string) error {
	// Générer une clé RSA 4096 bits
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("impossible de générer la clé RSA: %w", err)
	}

	// Encoder et sauvegarder la clé privée
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	if err := os.WriteFile(privateKeyPath, privateKeyPEM, 0600); err != nil {
		return fmt.Errorf("impossible d'écrire la clé privée: %w", err)
	}

	// Encoder et sauvegarder la clé publique
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("impossible de marshaler la clé publique: %w", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	if err := os.WriteFile(publicKeyPath, publicKeyPEM, 0644); err != nil {
		return fmt.Errorf("impossible d'écrire la clé publique: %w", err)
	}

	return nil
}

// SignFile signe un fichier et retourne la signature en base64
func (s *Signer) SignFile(filePath string) (string, string, error) {
	// Lire le contenu du fichier
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", fmt.Errorf("impossible de lire le fichier à signer (%s): %w", filePath, err)
	}

	// Calculer le hash SHA-256
	hash := sha256.Sum256(data)
	hashHex := fmt.Sprintf("%x", hash)

	// Signer le hash avec la clé privée RSA
	signature, err := rsa.SignPKCS1v15(
		rand.Reader,
		s.privateKey,
		crypto.SHA256,
		hash[:],
	)
	if err != nil {
		return "", "", fmt.Errorf("échec de la signature RSA: %w", err)
	}

	// Encoder la signature en base64
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	return signatureB64, hashHex, nil
}

// VerifySignature vérifie une signature RSA (utile pour les tests et les clients)
func (s *Signer) VerifySignature(data []byte, signatureB64 string) error {
	// Décoder la signature
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("signature base64 invalide: %w", err)
	}

	// Calculer le hash
	hash := sha256.Sum256(data)

	// Vérifier avec la clé publique
	return rsa.VerifyPKCS1v15(
		&s.privateKey.PublicKey,
		crypto.SHA256,
		hash[:],
		signature,
	)
}

// GetPublicKeyPEM retourne la clé publique en format PEM (pour distribution)
func (s *Signer) GetPublicKeyPEM() (string, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&s.privateKey.PublicKey)
	if err != nil {
		return "", fmt.Errorf("impossible d'exporter la clé publique: %w", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return string(publicKeyPEM), nil
}

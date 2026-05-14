package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
)

func getCipherBlock() (cipher.Block, error) {
	appKey := os.Getenv("APP_KEY")
	if appKey == "" {
		return nil, errors.New("APP_KEY is missing from environment")
	}

	appKey = strings.TrimPrefix(appKey, "base64:")
	keyBytes, err := base64.StdEncoding.DecodeString(appKey)
	if err != nil {
		return nil, errors.New("invalid APP_KEY format")
	}

	return aes.NewCipher(keyBytes)
}

// Encrypt encrypts a plaintext string using AES-256-GCM
func Encrypt(plaintext string) (string, error) {
	block, err := getCipherBlock()
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a ciphertext string using AES-256-GCM
func Decrypt(encryptedText string) (string, error) {
	block, err := getCipherBlock()
	if err != nil {
		return "", err
	}

	enc, err := base64.URLEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(enc) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]
	plaintextBytes, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintextBytes), nil
}

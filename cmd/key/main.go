package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func main() {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		slog.Error("Failed to generate random key", "error", err)
		os.Exit(1)
	}

	encodedKey := base64.StdEncoding.EncodeToString(key)
	appKeyStr := fmt.Sprintf("APP_KEY=base64:%s\n", encodedKey)

	// Check if .env exists
	envPath := ".env"
	content, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("No .env file found. Creating one and injecting APP_KEY...")
			os.WriteFile(envPath, []byte(appKeyStr), 0644)
			slog.Info("Successfully created .env and set APP_KEY")
			return
		}
		slog.Error("Failed to read .env", "error", err)
		os.Exit(1)
	}

	envStr := string(content)
	lines := strings.Split(envStr, "\n")
	keyExists := false

	for i, line := range lines {
		if strings.HasPrefix(line, "APP_KEY=") {
			lines[i] = strings.TrimSpace(appKeyStr)
			keyExists = true
			break
		}
	}

	if !keyExists {
		lines = append(lines, strings.TrimSpace(appKeyStr))
	}

	newEnv := strings.Join(lines, "\n")
	if !strings.HasSuffix(newEnv, "\n") {
		newEnv += "\n"
	}

	if err := os.WriteFile(envPath, []byte(newEnv), 0644); err != nil {
		slog.Error("Failed to update .env", "error", err)
		os.Exit(1)
	}

	slog.Info("Successfully generated new APP_KEY in .env!")
}

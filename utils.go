package main

import (
	"log/slog"
	"os"
	"fmt"

	"github.com/mbarek-hani/FluxHUB/services"
)

func EnsureKeyExists(privateKeyPath string) error {
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		slog.Info(fmt.Sprint("Generating RSA 4096 key pair..."))
		os.MkdirAll("./keys", 0700)
		publicKeyPath := "./keys/flux_hub_public.pem"
		if err := services.GenerateKeyPair(privateKeyPath, publicKeyPath); err != nil {
			return err
		}
		slog.Info(fmt.Sprintf("Keys generated: %s, %s", privateKeyPath, publicKeyPath))
	}
	return nil
}

func GetEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

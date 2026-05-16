package main

type Config struct {
	Port               string
	StoragePath        string
	ZipsPath           string
	PrivateKeyPath     string
	GinMode            string
	GithubClientID     string
	GithubClientSecret string
	GithubRedirectURL  string
}

func LoadConfig() Config {
	return Config{
		Port:               GetEnv("PORT", "8080"),
		StoragePath:        GetEnv("STORAGE_PATH", "./storage/clones"),
		ZipsPath:           GetEnv("ZIPS_PATH", "./storage/zips"),
		PrivateKeyPath:     GetEnv("PRIVATE_KEY_PATH", "./keys/flux_hub_private.pem"),
		GinMode:            GetEnv("GIN_MODE", "debug"),
		GithubClientID:     GetEnv("GITHUB_CLIENT_ID", ""),
		GithubClientSecret: GetEnv("GITHUB_CLIENT_SECRET", ""),
		GithubRedirectURL:  GetEnv("GITHUB_REDIRECT_URL", "http://localhost:8080/auth/github/callback"),
	}
}

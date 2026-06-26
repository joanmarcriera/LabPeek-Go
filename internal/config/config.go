package config

import (
	"os"
)

type Config struct {
	DBPath          string
	DataDir         string
	HTTPAddr        string
	AllowPublicScan bool
	AdminPassword   string
	AppName         string
}

func Load() *Config {
	return &Config{
		DBPath:          env("LABPEEK_DB", "./data/labpeek.db"),
		DataDir:         env("LABPEEK_DATA_DIR", "./data"),
		HTTPAddr:        env("LABPEEK_ADDR", ":8080"),
		AllowPublicScan: env("LABPEEK_ALLOW_PUBLIC_SCAN", "false") == "true",
		AdminPassword:   env("LABPEEK_ADMIN_PASSWORD", ""),
		AppName:         env("LABPEEK_APP_NAME", "LabPeek"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

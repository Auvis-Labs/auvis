package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	UpstreamURL  string
	AuditFile    string
}

func LoadConfig() Config {
	godotenv.Load()
	upstream := os.Getenv("IRIS_UPSTREAM")
	if upstream == "" {
		log.Fatal("IRIS_UPSTREAM environment variable is required")
	}

	port := os.Getenv("IRIS_PORT")
	if port == "" {
		port = "8080"
	}

	auditFile := os.Getenv("IRIS_AUDIT_FILE")
	if auditFile == "" {
		auditFile = "./audit.jsonl"
	}

	return Config{
		Port:        port,
		UpstreamURL: upstream,
		AuditFile:   auditFile,
	}
}
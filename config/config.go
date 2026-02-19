package config

import (
	"log"
	"os"
)

type Config struct {
	Port         string
	UpstreamURL  string
	AuditFile    string // we'll use later
}

func LoadConfig() Config {
	upstream := os.Getenv("IRIS_UPSTREAM")
	if upstream == "" {
		log.Fatal("IRIS_UPSTREAM environment variable is required (e.g. http://localhost:3000)")
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
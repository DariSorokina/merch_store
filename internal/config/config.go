package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var (
	LogLevel         string
	ServerRunAddress string
	DatabaseURI      string
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using default values")
	}

	LogLevel = os.Getenv("LOG_LEVEL")
	if LogLevel == "" {
		LogLevel = "info"
	}

	ServerRunAddress = os.Getenv("SERVER_RUN_ADDRESS")
	if ServerRunAddress == "" {
		ServerRunAddress = "0.0.0.0:8080"
	}

	DatabaseURI = os.Getenv("DATABASE_URI")
	if DatabaseURI == "" {
		DatabaseURI = "host=db user=postgres password=password dbname=shop sslmode=disable"
	}
}

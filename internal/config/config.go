package config

import (
	"os"
)

type Config struct {
	MongoURI string
	DBName   string
	APIKey   string
	Port     string
}

func LoadConfig() Config {
	return Config{
		MongoURI: os.Getenv("MONGO_URI"),
		DBName:   os.Getenv("egs"),
		APIKey:   os.Getenv("SECRET123"),
		Port:     os.Getenv("PORT"), 
	}
}

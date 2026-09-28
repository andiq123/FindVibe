package utils

import (
	"os"
)

func GetEnvOrDef(key, def string) string {
	value := os.Getenv(key)

	if value == "" {
		return def
	}

	return value
}

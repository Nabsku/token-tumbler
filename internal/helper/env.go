package helper

import (
	"os"
	"strings"
)

func CheckAndGetEnvVar(name string) bool {
	if name == "" {
		return false
	}
	value, ok := os.LookupEnv(name)
	return ok && strings.TrimSpace(value) != ""
}

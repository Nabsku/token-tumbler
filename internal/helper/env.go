package helper

import (
	"os"
)

func CheckAndGetEnvVar(name string) bool {
	if name == "" {
		return false
	}
	_, ok := os.LookupEnv(name)
	return ok
}

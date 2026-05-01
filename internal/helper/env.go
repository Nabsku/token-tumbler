package helper

import (
	"fmt"
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

func CheckEnvVars(vars ...string) error {
	var missingVars []string
	for _, v := range vars {
		if !CheckAndGetEnvVar(v) {
			missingVars = append(missingVars, v)
		}
	}
	if len(missingVars) > 0 {
		return fmt.Errorf("missing the following environment variables: %v", strings.Join(missingVars, ", "))
	}
	return nil
}

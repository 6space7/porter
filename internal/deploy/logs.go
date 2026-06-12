package deploy

import "strings"

func RedactSecrets(log string, secrets []string) string {
	redacted := log
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		redacted = strings.ReplaceAll(redacted, secret, "[REDACTED]")
	}
	return redacted
}

package services

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var placeholderPattern = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)

type RenderInput struct {
	ServiceID     string
	ContainerName string
	Hostname      string
}

type RenderedTemplate struct {
	Env       map[string]string
	Provides  map[string]string
	Generated map[string]string
}

type SecretGenerator interface {
	Generate(label string) (string, error)
}

type SecretGeneratorFunc func(label string) (string, error)

func (fn SecretGeneratorFunc) Generate(label string) (string, error) {
	return fn(label)
}

type StaticSecretGenerator string

func (generator StaticSecretGenerator) Generate(string) (string, error) {
	return string(generator), nil
}

type RandomSecretGenerator struct{}

func (RandomSecretGenerator) Generate(string) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func RenderTemplate(tmpl Template, input RenderInput, generator SecretGenerator) (RenderedTemplate, error) {
	if generator == nil {
		generator = RandomSecretGenerator{}
	}

	values := map[string]string{
		"SERVICE_HOST": input.ContainerName,
	}
	if input.Hostname != "" {
		values["SERVICE_DOMAIN"] = input.Hostname
		values["SERVICE_URL"] = "https://" + input.Hostname
	}

	env := map[string]string{}
	generated := map[string]string{}
	for _, key := range sortedKeys(tmpl.Variables) {
		value, err := expandGeneratedSecret(tmpl.Variables[key], key, generator, generated)
		if err != nil {
			return RenderedTemplate{}, err
		}
		env[key] = value
		values[key] = value
	}
	for _, key := range sortedKeys(env) {
		env[key] = expandKnownPlaceholders(env[key], values)
		values[key] = env[key]
	}

	provides := map[string]string{}
	for _, key := range sortedKeys(tmpl.Provides) {
		provides[key] = expandKnownPlaceholders(tmpl.Provides[key], values)
	}

	return RenderedTemplate{
		Env:       env,
		Provides:  provides,
		Generated: generated,
	}, nil
}

func expandGeneratedSecret(value, envKey string, generator SecretGenerator, generated map[string]string) (string, error) {
	var expandErr error
	expanded := placeholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		if expandErr != nil {
			return match
		}
		label := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		if !strings.HasPrefix(label, "SERVICE_PASSWORD_") {
			return match
		}
		secretLabel := strings.TrimPrefix(label, "SERVICE_PASSWORD_")
		secret, err := generator.Generate(secretLabel)
		if err != nil {
			expandErr = fmt.Errorf("generate %s: %w", secretLabel, err)
			return match
		}
		generated[envKey] = secret
		return secret
	})
	if expandErr != nil {
		return "", expandErr
	}
	return expanded, nil
}

func expandKnownPlaceholders(value string, values map[string]string) string {
	return placeholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		if replacement, ok := values[key]; ok {
			return replacement
		}
		return match
	})
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

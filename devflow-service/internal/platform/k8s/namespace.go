package k8s

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNamespaceProjectNameRequired     = errors.New("project name is required for namespace derivation")
	ErrNamespaceEnvironmentNameRequired = errors.New("environment name is required for namespace derivation")
	ErrNamespaceProjectNameInvalid      = errors.New("project name is invalid for namespace derivation")
	ErrNamespaceEnvironmentNameInvalid  = errors.New("environment name is invalid for namespace derivation")
	ErrNamespaceDerivedValueTooLong     = errors.New("derived namespace exceeds kubernetes length limit")
)

func DeriveNamespace(projectName, environmentName string) (string, error) {
	projectToken, err := normalizeNamespaceToken(projectName)
	if err != nil {
		if errors.Is(err, errNamespaceTokenEmpty) {
			return "", ErrNamespaceProjectNameRequired
		}
		return "", fmt.Errorf("%w: %v", ErrNamespaceProjectNameInvalid, err)
	}
	environmentToken, err := normalizeNamespaceToken(environmentName)
	if err != nil {
		if errors.Is(err, errNamespaceTokenEmpty) {
			return "", ErrNamespaceEnvironmentNameRequired
		}
		return "", fmt.Errorf("%w: %v", ErrNamespaceEnvironmentNameInvalid, err)
	}

	namespace := projectToken
	if environmentToken != "production" {
		namespace = projectToken + "-" + environmentToken
	}
	if len(namespace) > 63 {
		return "", ErrNamespaceDerivedValueTooLong
	}
	return namespace, nil
}

var errNamespaceTokenEmpty = errors.New("namespace token is empty")

func normalizeNamespaceToken(value string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "", errNamespaceTokenEmpty
	}
	trimmed = strings.ReplaceAll(trimmed, "_", "-")
	trimmed = strings.ReplaceAll(trimmed, " ", "-")

	builder := strings.Builder{}
	builder.Grow(len(trimmed))
	lastDash := false
	for _, r := range trimmed {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}

	normalized := strings.Trim(builder.String(), "-")
	if normalized == "" {
		return "", errNamespaceTokenEmpty
	}
	return normalized, nil
}

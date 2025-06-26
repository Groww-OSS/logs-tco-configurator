package kubernetes

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	maxRetries = 14
	baseDelay  = time.Second * 1
)

// FetchSecretValue fetches a secret and returns the value of the given key as a plain string
// It includes retry logic with exponential backoff for transient errors
func (k *K8sClient) FetchSecretValue(namespace, secretName, key string) (string, error) {

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1)) // Exponential backoff: 1s, 2s, 4s
			log.Debug().
				Str("namespace", namespace).
				Str("secret", secretName).
				Str("key", key).
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("retrying secret fetch after delay")
			time.Sleep(delay)
		}

		log.Debug().
			Str("namespace", namespace).
			Str("secret", secretName).
			Str("key", key).
			Int("attempt", attempt+1).
			Int("max_attempts", maxRetries+1).
			Msg("fetching secret value")

		secret, err := k.clientset.CoreV1().
			Secrets(namespace).
			Get(context.TODO(), secretName, metav1.GetOptions{})

		if err != nil {
			lastErr = fmt.Errorf("failed to get secret: %v", err)
			log.Debug().
				Str("namespace", namespace).
				Str("secret", secretName).
				Err(err).
				Msg("error fetching secret, will retry")
			continue
		}

		value, ok := secret.Data[key]
		if !ok {
			return "", fmt.Errorf("key %s not found in secret", key)
		}

		return string(value), nil
	}

	return "", fmt.Errorf("failed to get secret after %d attempts: %v", maxRetries+1, lastErr)
}

// UpdateSecretValue updates the value of the given key in the secret with the provided YAML value
// It includes retry logic with exponential backoff for transient errors
func (k *K8sClient) UpdateSecretValue(namespace, secretName, key, yamlValue string, dryRun bool) error {

	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {

		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1)) // Exponential backoff: 1s, 2s, 4s

			log.Debug().
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("retrying secret update after delay")
			time.Sleep(delay)
		}

		log.Trace().
			Str("namespace", namespace).
			Str("secret", secretName).
			Str("key", key).
			Int("attempt", attempt+1).
			Int("max_attempts", maxRetries+1).
			Msg("Updating Secret")

		// Fetch the secret
		secret, err := k.clientset.CoreV1().
			Secrets(namespace).
			Get(context.TODO(), secretName, metav1.GetOptions{})

		if err != nil {
			lastErr = fmt.Errorf("failed to get secret: %v", err)
			log.Warn().
				Str("namespace", namespace).
				Str("secret", secretName).
				Err(err).
				Msg("error getting secret for update, will retry")
			continue
		}

		secret.Data[key] = []byte(yamlValue)

		updateOptions := metav1.UpdateOptions{}

		if dryRun {
			updateOptions.DryRun = []string{metav1.DryRunAll}
		}

		// Update the secret
		_, err = k.clientset.CoreV1().
			Secrets(namespace).
			Update(context.TODO(), secret, updateOptions)

		if err != nil {
			lastErr = fmt.Errorf("failed to update secret: %v", err)
			log.Warn().
				Str("namespace", namespace).
				Str("secret", secretName).
				Err(err).
				Msg("error updating secret, will retry")
			continue
		}

		log.Trace().
			Str("namespace", namespace).
			Str("secret", secretName).
			Str("key", key).
			Str("dry_run", fmt.Sprintf("%v", dryRun)).
			Msg("Secret updated successfully")

		log.Trace().
			Msg(fmt.Sprintf("New Promtail Config:\n%v", yamlValue))

		return nil
	}

	return fmt.Errorf("failed to update secret after %d attempts: %v", maxRetries+1, lastErr)
}

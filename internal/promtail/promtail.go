package promtail

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v2"
)

// LoadConfig loads the promtail configuration from a YAML string.

func New(yamlStr string) (*PromtailConfig, error) {

	log.Trace().
		Msg("parsing promtail config from string")

	var config PromtailConfig

	err := yaml.Unmarshal([]byte(yamlStr), &config)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
	}

	return &config, nil
}

// ToYAML converts the promtail configuration back to a YAML string.
func (p *PromtailConfig) ToYAML() (string, error) {

	log.Trace().
		Msg("Marshaling Promtail config YAML")

	yamlBytes, err := yaml.Marshal(p)

	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %v", err)
	}

	return string(yamlBytes), nil
}

// ValidateConfig validates the promtail configuration by writing it to a temporary file and running promtail -check-syntax
func (p *PromtailConfig) ValidateConfig(promtailBin string) error {

	tempFile := "promtail-config-*.yaml"

	log.Trace().
		Msg("Validating promtail config")

	yamlStr, err := p.ToYAML()

	if err != nil {
		return fmt.Errorf("failed to convert to YAML: %v", err)
	}

	// create a temp file
	tmpFile, err := os.CreateTemp("", tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	_, err = tmpFile.WriteString(yamlStr)
	if err != nil {
		return fmt.Errorf("failed to write to temp file: %v", err)
	}
	err = tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close temp file: %v", err)
	}
	log.Trace().
		Str("tempFile", tmpFile.Name()).
		Msg("Created temp file")

	log.Trace().
		Msg("Running promtail -check-syntax")

	// execute promtail config test

	cmd := exec.Command(
		promtailBin,
		"-check-syntax",
		"--config.file",
		tmpFile.Name(),
	)

	output, err := cmd.Output()

	if err != nil {
		log.Fatal().
			Err(err).
			Str("output", string(output)).
			Msg("Failed to run promtail -check-syntax")

		return fmt.Errorf("failed to run promtail -check-syntax: %v", err)
	}

	log.Trace().
		Str("output", string(output)).
		Msg("Promtail config validation successful")

	defer os.Remove(tmpFile.Name())

	return nil
}

package pipeline

import (
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

var envVarRegex = regexp.MustCompile(`\$\{[A-Za-z_][A-Za-z0-9_]*\}`)

// ResolveEnvVars parses the input YAML bytes, scans for ${ENV_VAR} patterns,
// replaces them with values from the environment, and returns the expanded bytes.
//
// Example:
//
//	api_key: "${API_KEY_SECRET}" -> api_key: "my-real-secret"
func ResolveEnvVars(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	// We use regexp replacement to strictly target `${VAR}` syntax instead of
	// standard `os.ExpandEnv` which would also expand `$VAR` (which might break
	// certain regexes or dollar signs in inline scripts or YAML maps).
	// Strictly resolving `${ENV_VAR}` ensures maximum safety.
	expanded := envVarRegex.ReplaceAllFunc(data, func(match []byte) []byte {
		// Strip "${" from start and "}" from end
		varName := string(match[2 : len(match)-1])
		return []byte(os.Getenv(varName))
	})

	return expanded
}

// ParsePipelineConfig resolves environment variables on the input bytes,
// and unmarshals it into a PipelineConfig structure.
func ParsePipelineConfig(data []byte) (*PipelineConfig, error) {
	resolved := ResolveEnvVars(data)
	var cfg PipelineConfig
	if err := yaml.Unmarshal(resolved, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadPipelineConfig reads the YAML configuration file at the given path,
// resolves any environment variables, and parses it into a PipelineConfig.
func LoadPipelineConfig(path string) (*PipelineConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePipelineConfig(data)
}


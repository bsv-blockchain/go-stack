package loaders_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/bsv-blockchain/go-overlay-services/pkg/server/config/loaders"
)

// FuzzSetConfigFilePath fuzzes the SetConfigFilePath method to test file path validation
// and extension parsing. This validates the robustness of path handling and extension
// detection against malformed or unusual file paths.
func FuzzSetConfigFilePath(f *testing.F) {
	// Seed corpus with valid file paths
	f.Add("config.yaml")
	f.Add("config.yml")
	f.Add("config.json")
	f.Add("config.env")
	f.Add("config.dotenv")
	f.Add("/path/to/config.yaml")
	f.Add("./relative/path/config.json")
	f.Add("/absolute/path/to/config.env")

	// Seed corpus with invalid file paths
	f.Add("config.txt")
	f.Add("config.xml")
	f.Add("config.toml")
	f.Add("config")
	f.Add("config.")
	f.Add(".yaml")
	f.Add("")

	// Edge cases
	f.Add("config.yaml.backup")
	f.Add("config..yaml")
	f.Add("config.YAML")
	f.Add("config.YML")
	f.Add("config.JSON")
	f.Add("config.yaml\x00")
	f.Add("config\nyaml")
	f.Add("config\tyaml")
	f.Add("../../../../etc/passwd")
	f.Add("/etc/passwd")
	f.Add("C:\\Windows\\System32\\config.yaml")

	f.Fuzz(func(t *testing.T, filePath string) {
		// Create a new loader for each fuzz iteration
		l := loaders.NewLoader(NewExporterTestConfig, "TEST")

		// The function should never panic
		err := l.SetConfigFilePath(filePath)

		// Validate the behavior based on the file extension
		// Supported extensions: yaml, yml, json, dotenv, env
		supportedExts := []string{"yaml", "yml", "json", "dotenv", "env"}

		// Extract the extension from the file path
		ext := ""
		if lastDot := strings.LastIndex(filePath, "."); lastDot >= 0 && lastDot < len(filePath)-1 {
			ext = filePath[lastDot+1:]
		}

		// Check if the extension is supported
		isSupported := false
		for _, supportedExt := range supportedExts {
			if ext == supportedExt {
				isSupported = true
				break
			}
		}

		// Invariant: if extension is supported, no error should be returned
		if isSupported && err != nil {
			t.Errorf("SetConfigFilePath(%q) returned error %v for supported extension %q", filePath, err, ext)
		}

		// Invariant: if extension is not in supported list and not empty path, error should be returned
		if !isSupported && filePath != "" && ext != "" && err == nil {
			t.Errorf("SetConfigFilePath(%q) returned nil error for unsupported extension %q", filePath, ext)
		}
	})
}

// FuzzLoaderWithMalformedYAML fuzzes the Loader with malformed YAML content to test parsing robustness.
func FuzzLoaderWithMalformedYAML(f *testing.F) {
	// Seed corpus with valid YAML
	f.Add("b_with_long_name: 42\nc_sub_config:\n  d_nested_field: test\n")
	f.Add("b_with_long_name: 1\n")
	f.Add("c_sub_config:\n  d_nested_field: hello\n")

	// Seed corpus with malformed YAML
	f.Add("b_with_long_name: [unclosed")
	f.Add("b_with_long_name:\n  - invalid\n  indentation")
	f.Add("b_with_long_name: 'unclosed string")
	f.Add(": no key")
	f.Add("no value:")
	f.Add("tab\tseparated: value")
	f.Add("b_with_long_name: 999999999999999999999999999999999999")

	// Edge cases
	f.Add("")
	f.Add("\n")
	f.Add("   \n   \n")
	f.Add("\x00\x00\x00")
	f.Add("---\n...\n")
	f.Add("!!binary")

	f.Fuzz(func(t *testing.T, yamlContent string) {
		// Create a temporary YAML file
		tmpDir := t.TempDir()
		configFilePath := fmt.Sprintf("%s/config.yaml", tmpDir)
		if err := os.WriteFile(configFilePath, []byte(yamlContent), 0o600); err != nil {
			t.Skip("Failed to write temp file")
		}

		// Create loader and set the config file path
		l := loaders.NewLoader(NewExporterTestConfig, "TEST")
		if err := l.SetConfigFilePath(configFilePath); err != nil {
			t.Skip("Failed to set config file path")
		}

		// The Load function should never panic, even with malformed YAML
		// It may return an error, which is acceptable
		_, _ = l.Load()
	})
}

// FuzzLoaderWithMalformedJSON fuzzes the Loader with malformed JSON content to test parsing robustness.
func FuzzLoaderWithMalformedJSON(f *testing.F) {
	// Seed corpus with valid JSON
	f.Add(`{"b_with_long_name": 42, "c_sub_config": {"d_nested_field": "test"}}`)
	f.Add(`{"b_with_long_name": 1}`)
	f.Add(`{"c_sub_config": {"d_nested_field": "hello"}}`)

	// Seed corpus with malformed JSON
	f.Add(`{"b_with_long_name": }`)
	f.Add(`{"b_with_long_name": [unclosed`)
	f.Add(`{"b_with_long_name": "unclosed string}`)
	f.Add(`{: "no key"}`)
	f.Add(`{"no value":}`)
	f.Add(`{"b_with_long_name": 999999999999999999999999999999999999}`)
	f.Add(`{"trailing": "comma",}`)

	// Edge cases
	f.Add("")
	f.Add("{}")
	f.Add("null")
	f.Add("[]")
	f.Add(`{"a": null}`)
	f.Add("\x00\x00\x00")

	f.Fuzz(func(t *testing.T, jsonContent string) {
		// Create a temporary JSON file
		tmpDir := t.TempDir()
		configFilePath := fmt.Sprintf("%s/config.json", tmpDir)
		if err := os.WriteFile(configFilePath, []byte(jsonContent), 0o600); err != nil {
			t.Skip("Failed to write temp file")
		}

		// Create loader and set the config file path
		l := loaders.NewLoader(NewExporterTestConfig, "TEST")
		if err := l.SetConfigFilePath(configFilePath); err != nil {
			t.Skip("Failed to set config file path")
		}

		// The Load function should never panic, even with malformed JSON
		// It may return an error, which is acceptable
		_, _ = l.Load()
	})
}

// FuzzLoaderWithMalformedEnv fuzzes the Loader with malformed .env content to test parsing robustness.
func FuzzLoaderWithMalformedEnv(f *testing.F) {
	// Seed corpus with valid .env
	f.Add("TEST_B_WITH_LONG_NAME=42\nTEST_C_SUB_CONFIG_D_NESTED_FIELD=test\n")
	f.Add("TEST_B_WITH_LONG_NAME=1\n")
	f.Add("TEST_C_SUB_CONFIG_D_NESTED_FIELD=hello\n")

	// Seed corpus with malformed .env
	f.Add("TEST_B_WITH_LONG_NAME=")
	f.Add("=value_without_key")
	f.Add("NO_EQUALS_SIGN")
	f.Add("TEST_B_WITH_LONG_NAME=\"unclosed quote")
	f.Add("TEST_B_WITH_LONG_NAME='unclosed quote")
	f.Add("TEST_B_WITH_LONG_NAME=value with spaces")
	f.Add("KEY==double_equals")

	// Edge cases
	f.Add("")
	f.Add("\n\n\n")
	f.Add("#comment only")
	f.Add("# Comment\nTEST_B_WITH_LONG_NAME=1")
	f.Add("   SPACES_BEFORE=value")
	f.Add("TABS\tBETWEEN=value")
	f.Add("\x00\x00\x00")

	f.Fuzz(func(t *testing.T, envContent string) {
		// Create a temporary .env file
		tmpDir := t.TempDir()
		configFilePath := fmt.Sprintf("%s/config.env", tmpDir)
		if err := os.WriteFile(configFilePath, []byte(envContent), 0o600); err != nil {
			t.Skip("Failed to write temp file")
		}

		// Create loader and set the config file path
		l := loaders.NewLoader(NewExporterTestConfig, "TEST")
		if err := l.SetConfigFilePath(configFilePath); err != nil {
			t.Skip("Failed to set config file path")
		}

		// The Load function should never panic, even with malformed .env
		// It may return an error, which is acceptable
		_, _ = l.Load()
	})
}

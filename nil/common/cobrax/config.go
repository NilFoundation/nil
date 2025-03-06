package cobrax

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// AddConfigFlag adds a flag to the flag set to specify a config file.
// It doesn't attach the flag to any variable because normally GetConfigNameFromArgs is used.
func AddConfigFlag(fset *pflag.FlagSet) {
	fset.StringP("config", "c", "", "config file")
}

// GetConfigNameFromArgs searches for a config file name in the command line arguments.
// Generally, it should be called before argument parsing because the latter depends on the config (default values).
func GetConfigNameFromArgs() string {
	for i, f := range os.Args[:len(os.Args)-1] {
		if f == "--config" || f == "-c" {
			return os.Args[i+1]
		}
	}
	return ""
}

// LoadConfigFromFile reads a YAML file and unmarshals it into the destination.
// If the file name is empty, it does nothing.
// Arg dest must be a non-nil pointer (initialized with defaults).
func LoadConfigFromFile[T any](name string, dest *T) error {
	if name == "" {
		return nil
	}

	data, err := os.ReadFile(name)
	if err != nil {
		return fmt.Errorf("can't read config %s: %w", name, err)
	}

	if err := yaml.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("can't parse config %s: %w", name, err)
	}
	return nil
}

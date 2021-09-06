package bass

import (
	"fmt"
	"os"

	"github.com/adrg/xdg"
)

const PlatformOS Symbol = "os"

var LinuxPlatform = Bindings{
	PlatformOS: String("linux"),
}.Scope()

var WindowsPlatform = Bindings{
	PlatformOS: String("windows"),
}.Scope()

var DarwinPlatform = Bindings{
	PlatformOS: String("darwin"),
}.Scope()

// Config is set by the user and read by the Bass language and runtimes which
// run on the same machine.
type Config struct {
	Runtimes []RuntimeConfig `json:"runtimes"`
}

// RuntimeConfig associates a platform object to a runtime command to run.
//
// Additional configuration may be specified; it will be read from the runtime
// by finding the config associated to the platform on the workload it receives.
type RuntimeConfig struct {
	Platform *Scope `json:"platform"`
	Runtime  string `json:"runtime"`
	Config   *Scope `json:"config,omitempty"`
}

// Matches returns true if its runtime should be used for the given platform.
func (config RuntimeConfig) Matches(platform *Scope) bool {
	return config.Platform.Equal(platform)
}

// LoadConfig loads a Config from the JSON file at the given path.
func LoadConfig(defaultConfig Config) (*Config, error) {
	path, err := xdg.ConfigFile("bass/config.json")
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &defaultConfig, nil
		}

		return nil, err
	}

	var config Config
	err = UnmarshalJSON(payload, &config)
	if err != nil {
		return nil, err
	}

	return &config, err
}

// RuntimeConfig fetches the configuration for the given platform and decodes it
// into dest.
func (config *Config) RuntimeConfig(platform *Scope, dest interface{}) error {
	for _, runtime := range config.Runtimes {
		if runtime.Matches(platform) {
			return runtime.Config.Decode(dest)
		}
	}

	return nil
}

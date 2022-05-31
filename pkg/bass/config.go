package bass

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/adrg/xdg"
)

// Config is set by the user and read by the Bass language and runtimes which
// run on the same machine.
type Config struct {
	Runtimes []RuntimeConfig `json:"runtimes"`
}

// RuntimeConfig associates a platform object to a runtime command to run.
//
// Additional configuration may be specified; it will be read from the runtime
// by finding the config associated to the platform on the thunk it receives.
type RuntimeConfig struct {
	Platform Platform     `json:"platform"`
	Runtime  string       `json:"runtime"`
	Addrs    RuntimeAddrs `json:"addrs,omitempty"`
	Config   *Scope       `json:"config,omitempty"`
}

// RuntimeAddrs contains addresses of various services.
type RuntimeAddrs map[string]*url.URL

func (addrs RuntimeAddrs) Service(name string) (*url.URL, bool) {
	if addrs == nil {
		return nil, false
	}

	u, found := addrs[name]
	return u, found
}

func (addrs *RuntimeAddrs) UnmarshalJSON(p []byte) error {
	newAddrs := make(map[string]*url.URL)

	var m map[string]string
	if err := json.Unmarshal(p, &m); err != nil {
		return fmt.Errorf("malformed addrs: %w", err)
	}

	for name, urlStr := range m {
		u, err := url.Parse(urlStr)
		if err != nil {
			return fmt.Errorf("addr %q: %w", name, err)
		}

		newAddrs[name] = u
	}

	*addrs = newAddrs

	return nil
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

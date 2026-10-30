package config

import (
	"fmt"
	"os"
	"time"

	"go.yaml.in/yaml/v2"
)

type Config struct {
	ServerConfig           *ServerConfig `yaml:"server"`
	Log                    Log           `yaml:"log"`
	Services               []*Service    `yaml:"services"`
	DisableReloadByRequest bool          `yaml:"disable_reload_by_request"`
	Cache                  Cache         `yaml:"cache"`
}

// Cache keeps the answer of each source for a while, so clients polling together don't each hit the registry
type Cache struct {
	TTL *time.Duration `yaml:"ttl"`
}

const defaultCacheTTL = 5 * time.Second

type Log struct {
	Level   string `yaml:"level"`
	NoColor bool   `yaml:"no_color"`
	InJson  bool   `yaml:"in_json"`
}

func (c *Log) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Log
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}

	return nil
}

func (u *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Config
	err := unmarshal((*plain)(u))
	if err != nil {
		return err
	}
	if u.ServerConfig == nil {
		u.ServerConfig = &ServerConfig{
			ListenAddr: ":8080",
		}
	}
	if len(u.Services) == 0 {
		return fmt.Errorf("services is empty")
	}

	ttl := defaultCacheTTL
	if u.Cache.TTL != nil {
		ttl = *u.Cache.TTL
	}
	if ttl < 0 {
		return fmt.Errorf("cache ttl is negative")
	}
	for _, svc := range u.Services {
		if svc.CacheTTL == nil {
			svc.CacheTTL = &ttl
		}
	}
	return nil
}

func ReadConfig(configPath string) (*Config, error) {
	cnf := &Config{}
	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(b, cnf)
	if err != nil {
		return nil, err
	}
	return cnf, nil
}

// ReadConfigStrict reads the config like ReadConfig but also refuses the keys the config doesn't know:
// ReadConfig ignores them, so a misspelled key silently keeps its default.
func ReadConfigStrict(configPath string) (*Config, error) {
	cnf := &Config{}
	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	err = yaml.UnmarshalStrict(b, cnf)
	if err != nil {
		return nil, err
	}
	return cnf, nil
}

func WriteConfig(cnf *Config, configPath string) error {
	b, err := yaml.Marshal(cnf)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, b, 0644)
}

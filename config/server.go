package config

import (
	"fmt"
)

type ServerConfig struct {
	Tls        *ServerTls `yaml:"tls"`
	ListenAddr string     `yaml:"listen_addr"`
	// users allowed to call the server, every endpoint but /healthz then asks for one of them; empty leaves it open
	BasicAuth []BasicAuthUser `yaml:"basic_auth"`
}

func (u *ServerConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain ServerConfig
	err := unmarshal((*plain)(u))
	if err != nil {
		return err
	}

	if u.ListenAddr == "" {
		u.ListenAddr = ":8080"
	}
	seen := make(map[string]bool)
	for _, user := range u.BasicAuth {
		if seen[user.Username] {
			return fmt.Errorf("server basic_auth: username %s is given twice", user.Username)
		}
		seen[user.Username] = true
	}
	return nil
}

type ServerTls struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

func (u *ServerTls) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain ServerTls
	err := unmarshal((*plain)(u))
	if err != nil {
		return err
	}
	if u.CertFile == "" || u.KeyFile == "" {
		return fmt.Errorf("server tls: cert_file and key_file are required")
	}
	return nil
}

type BasicAuthUser struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func (u *BasicAuthUser) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain BasicAuthUser
	err := unmarshal((*plain)(u))
	if err != nil {
		return err
	}
	if u.Username == "" || u.Password == "" {
		return fmt.Errorf("server basic_auth: username and password are required")
	}
	return nil
}

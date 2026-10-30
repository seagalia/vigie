package config

import (
	"fmt"
)

type ServerConfig struct {
	Tls        *ServerTls `yaml:"tls"`
	ListenAddr string     `yaml:"listen_addr"`
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

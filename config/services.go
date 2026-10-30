package config

import (
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/seagalia/vigie/logging"
)

type Service struct {
	Name        string             `yaml:"name"`
	DefaultPort uint32             `yaml:"default_port"`
	DNS         *ServiceDNS        `yaml:"dns"`
	File        *ServiceFile       `yaml:"file"`
	URL         *ServiceURL        `yaml:"url"`
	Kubernetes  *ServiceKubernetes `yaml:"kubernetes"`
	// how long an answer of the source is reused; nil takes cache.ttl of the file once it is read
	CacheTTL *time.Duration `yaml:"cache_ttl"`
}

func (u *Service) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Service
	err := unmarshal((*plain)(u))
	entry := logging.Component("service")
	if err != nil {
		return err
	}
	if u.Name == "" {
		return fmt.Errorf("service name is empty")
	}
	if u.DefaultPort == 0 {
		u.DefaultPort = 80
		entry.Warn().Msgf("service %s default port is empty, use 80 as default", u.Name)
	}
	if u.CacheTTL != nil && *u.CacheTTL < 0 {
		return fmt.Errorf("service %s cache_ttl is negative", u.Name)
	}
	hasSource := u.DNS != nil || u.File != nil || u.URL != nil || u.Kubernetes != nil
	if !hasSource {
		return fmt.Errorf("service dns, file, url and kubernetes are empty, you must specify one of them")
	}
	return nil
}

type ServiceURL struct {
	Url       string `yaml:"url"`
	BasicAuth struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"basic_auth"`
	InsecureSkipVerify bool     `yaml:"insecure_skip_verify"`
	UrlParsed          *url.URL `yaml:"-"`
}

func (u *ServiceURL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain ServiceURL
	err := unmarshal((*plain)(u))
	if err != nil {
		return err
	}
	if u.Url == "" {
		return fmt.Errorf("service url is empty")
	}
	u.UrlParsed, err = url.Parse(u.Url)
	if err != nil {
		return fmt.Errorf("service url is invalid: %s", err)
	}

	return nil
}

type ServiceDNS struct {
	Name       string `yaml:"name"`
	SrvProto   string `yaml:"srv_proto"`
	SrvService string `yaml:"srv_service"`
	Type       string `yaml:"type"`
	// host:port of the DNS server to query instead of the system resolver, Consul's on port 8600 for instance
	Resolver string `yaml:"resolver"`
}

func (u *ServiceDNS) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain ServiceDNS
	entry := logging.Component("service_dns")
	err := unmarshal((*plain)(u))
	if err != nil {
		return err
	}
	if u.Name == "" {
		return fmt.Errorf("service dns name is empty")
	}
	if u.Type == "" {
		u.Type = "IP"
	}
	u.Type = strings.ToUpper(u.Type)
	if !slices.Contains([]string{"IP", "TXT", "SRV"}, u.Type) {
		return fmt.Errorf("service dns lookup is invalid, valid are IP, TXT, SRV")
	}

	if u.SrvProto == "" && u.Type == "SRV" {
		entry.Warn().Msgf("service %s srv proto is empty, use tcp as default", u.Name)
		u.SrvProto = "tcp"
	}
	if u.SrvService == "" && u.Type == "SRV" {
		entry.Warn().Msgf("service %s srv service is empty, use http as default", u.Name)
		u.SrvService = "http"
	}

	if u.Resolver == "" {
		return nil
	}
	host, port, err := net.SplitHostPort(u.Resolver)
	if err != nil {
		return fmt.Errorf("service dns resolver is invalid: %s", err)
	}
	if host == "" || port == "" {
		return fmt.Errorf("service dns resolver is invalid: %s needs a host and a port", u.Resolver)
	}
	return nil
}

type ServiceFile struct {
	Path string `yaml:"path"`
}

// ServiceKubernetes reads the ready endpoints of a Kubernetes Service from its EndpointSlices
type ServiceKubernetes struct {
	Namespace string `yaml:"namespace"`
	Service   string `yaml:"service"`
	PortName  string `yaml:"port_name"`
	// empty uses $KUBECONFIG, then ~/.kube/config, then the in-cluster service account
	Kubeconfig string `yaml:"kubeconfig"`
	Context    string `yaml:"context"`
}

func (u *ServiceKubernetes) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain ServiceKubernetes
	err := unmarshal((*plain)(u))
	if err != nil {
		return err
	}
	if u.Service == "" {
		return fmt.Errorf("service kubernetes service is empty")
	}
	if u.Namespace == "" {
		u.Namespace = "default"
	}
	return nil
}

package services

import (
	"context"
	"fmt"
	"time"

	"github.com/seagalia/vigie/config"

	"net"
	"strings"

	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
)

type DNS struct {
	defaultPort uint32
	cnf         *config.ServiceDNS
	resolver    *net.Resolver
}

func NewDNS(cnf *config.ServiceDNS, defaultPort uint32) *DNS {
	return &DNS{
		cnf:         cnf,
		defaultPort: defaultPort,
		resolver:    newResolver(cnf.Resolver),
	}
}

// newResolver queries the given server over tcp, so a long answer, such as a Consul service with many instances,
// is never cut to what fits in a udp packet
func newResolver(server string) *net.Resolver {
	if server == "" {
		return net.DefaultResolver
	}
	dialer := &net.Dialer{}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", server)
		},
	}
}

func (s *DNS) Resolve() ([]*eds.Endpoint, error) {
	ctx := context.Background()
	// a given server can be down, the system resolver has its own timeouts
	if s.cnf.Resolver != "" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
	}

	switch s.cnf.Type {
	case "SRV":
		return s.resolveLookupSRV(ctx)
	case "TXT":
		return s.resolveLookupTxt(ctx)
	}
	return s.resolveLookupIP(ctx)
}

func (s *DNS) resolveLookupIP(ctx context.Context) ([]*eds.Endpoint, error) {
	endpoints := make([]*eds.Endpoint, 0)
	ips, err := s.resolver.LookupIP(ctx, "ip", s.cnf.Name)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve on lookup IP: %w", err)
	}

	for _, ip := range ips {
		endpoints = append(endpoints, &eds.Endpoint{
			Ip:   ip.String(),
			Port: s.defaultPort,
		})
	}

	return endpoints, nil
}

func (s *DNS) resolveLookupSRV(ctx context.Context) ([]*eds.Endpoint, error) {
	endpoints := make([]*eds.Endpoint, 0)
	_, addrs, err := s.resolver.LookupSRV(ctx, s.cnf.SrvService, s.cnf.SrvProto, s.cnf.Name)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve on lookup SRV: %w", err)
	}

	for _, addr := range addrs {
		port := uint32(addr.Port)
		if port == 0 {
			port = s.defaultPort
		}
		ips, err := s.resolver.LookupIP(ctx, "ip", addr.Target)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve on lookup IP for target %s: %w", addr.Target, err)
		}
		for _, ip := range ips {
			endpoints = append(endpoints, &eds.Endpoint{
				Ip:                  ip.String(),
				Port:                port,
				LoadBalancingWeight: uint32(addr.Weight),
			})
		}
	}

	return endpoints, nil
}

func (s *DNS) resolveLookupTxt(ctx context.Context) ([]*eds.Endpoint, error) {
	endpoints := make([]*eds.Endpoint, 0)
	lines, err := s.resolver.LookupTXT(ctx, s.cnf.Name)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve on lookup TXT: %w", err)
	}
	for _, line := range lines {
		endpointLine, isEds := strings.CutPrefix(line, "eds=")
		if !isEds {
			continue
		}
		endpoint, err := urlToEndpoint(endpointLine, s.defaultPort)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}

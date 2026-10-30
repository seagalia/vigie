package services

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
)

type Service interface {
	Resolve() ([]*eds.Endpoint, error)
}

func urlToEndpoint(line string, defaultPort uint32) (*eds.Endpoint, error) {
	if !strings.Contains(line, "://") {
		line = "any://" + line
	}
	u, err := url.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	weight := 0
	if u.Fragment != "" {
		weight, err = strconv.Atoi(u.Fragment)
		if err != nil {
			return nil, fmt.Errorf("invalid weight: %w", err)
		}
	}
	host, portRaw, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}
	port := uint64(defaultPort)
	if portRaw != "" {
		port, err = strconv.ParseUint(portRaw, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}

	return &eds.Endpoint{
		// protobuf strings must be valid utf-8, a byte that isn't would fail the whole answer instead of this endpoint
		Ip:                  strings.ToValidUTF8(host, "\uFFFD"),
		Port:                uint32(port),
		LoadBalancingWeight: uint32(weight),
	}, nil
}

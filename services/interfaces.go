package services

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type IpAddr struct {
	IP                  string `json:"ip"`
	Port                uint32 `json:"port"`
	LoadBalancingWeight uint32 `json:"load_balancing_weight"`
}

type Service interface {
	Resolve() ([]IpAddr, error)
}

func urlToIpAddr(line string, defaultPort uint32) (IpAddr, error) {
	if !strings.Contains(line, "://") {
		line = "any://" + line
	}
	u, err := url.Parse(line)
	if err != nil {
		return IpAddr{}, fmt.Errorf("invalid url: %w", err)
	}
	weight := 0
	if u.Fragment != "" {
		weight, err = strconv.Atoi(u.Fragment)
		if err != nil {
			return IpAddr{}, fmt.Errorf("invalid weight: %w", err)
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
			return IpAddr{}, fmt.Errorf("invalid port: %w", err)
		}
	}

	return IpAddr{
		IP:                  host,
		Port:                uint32(port),
		LoadBalancingWeight: uint32(weight),
	}, nil
}

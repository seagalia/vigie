package services

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/seagalia/vigie/config"
)

type File struct {
	path        string
	defaultPort uint32
}

func NewFile(cnf *config.ServiceFile, defaultPort uint32) *File {
	return &File{
		path:        cnf.Path,
		defaultPort: defaultPort,
	}
}

func (s *File) Resolve() ([]IpAddr, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	ips := make([]IpAddr, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ipAddr, err := urlToIpAddr(line, s.defaultPort)
		if err != nil {
			return nil, err
		}
		ips = append(ips, ipAddr)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("cannot scan file: %w", err)
	}
	return ips, nil
}

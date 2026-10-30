package services

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/seagalia/vigie/config"

	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
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

func (s *File) Resolve() ([]*eds.Endpoint, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	endpoints := make([]*eds.Endpoint, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		endpoint, err := urlToEndpoint(line, s.defaultPort)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("cannot scan file: %w", err)
	}
	return endpoints, nil
}

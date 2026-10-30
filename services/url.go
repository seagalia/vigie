package services

import (
	"bufio"
	"crypto/tls"
	"fmt"

	"github.com/seagalia/vigie/config"

	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type URL struct {
	urlParse    *url.URL
	client      *http.Client
	username    string
	password    string
	defaultPort uint32
}

func NewURL(cnf *config.ServiceURL, defaultPort uint32) *URL {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cnf.InsecureSkipVerify,
		},
	}
	return &URL{
		urlParse: cnf.UrlParsed,
		client: &http.Client{
			Transport: transport,
			Timeout:   time.Second * 8,
		},
		username:    cnf.BasicAuth.Username,
		password:    cnf.BasicAuth.Password,
		defaultPort: defaultPort,
	}
}

func (s *URL) Resolve() ([]IpAddr, error) {
	req, err := http.NewRequest("GET", s.urlParse.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}
	if s.username != "" && s.password != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errMess := "no content error"
		b, err := io.ReadAll(resp.Body)
		if err == nil {
			errMess = string(b)
		}
		return nil, fmt.Errorf("bad status code: %d, %s", resp.StatusCode, errMess)
	}

	scanner := bufio.NewScanner(resp.Body)
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
	return ips, nil
}

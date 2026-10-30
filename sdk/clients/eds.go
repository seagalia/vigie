// Package clients builds clients of the services of Vigie.
package clients

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/seagalia/vigie/sdk/vigie/eds/v1/edsconnect"
)

// Options of the connection to Vigie; the zero value connects without TLS and without credentials.
type Options struct {
	// TLSConfig is used with an https base URL; nil trusts the CAs of the system.
	TLSConfig *tls.Config
	// Username and Password are sent as basic auth on each call when either is set, for a Vigie with
	// server.basic_auth.
	Username string
	Password string
}

// NewEndpointDiscovery returns a client of the EDS service of Vigie at baseURL, http(s)://host[:port][/prefix]. It
// speaks gRPC over HTTP/2: unencrypted (h2c) for http, over TLS for https. A path is put before the procedure, for a
// Vigie behind a reverse proxy.
func NewEndpointDiscovery(baseURL string, opts Options) (edsconnect.EndpointDiscoveryServiceClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse vigie url %q: %w", baseURL, err)
	}

	protocols := &http.Protocols{}
	// the default transport brings the proxy of the environment and the dial, handshake and idle timeouts
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Protocols = protocols
	switch parsed.Scheme {
	case "http":
		protocols.SetUnencryptedHTTP2(true)
	case "https":
		protocols.SetHTTP2(true)
		transport.TLSClientConfig = opts.TLSConfig
	default:
		return nil, fmt.Errorf("vigie url %q: scheme %q is neither http nor https", baseURL, parsed.Scheme)
	}

	var interceptors []connect.ClientInterceptor
	hasCredentials := opts.Username != "" || opts.Password != ""
	if hasCredentials {
		interceptors = append(interceptors, basicAuthInterceptor(opts.Username, opts.Password))
	}

	httpClient := &http.Client{Transport: transport}
	connectTransport := connecthttp.NewTransport(httpClient, baseURL, connecthttp.WithGRPC())
	return edsconnect.NewEndpointDiscoveryServiceClient(connect.NewClient(connectTransport, interceptors...)), nil
}

func basicAuthInterceptor(username, password string) connect.ClientInterceptor {
	authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	return func(next connect.ClientFunc) connect.ClientFunc {
		return func(ctx context.Context, spec connect.Spec) (connect.ClientStream, error) {
			// connect.Client attaches a CallInfo to ctx before running its interceptors
			callInfo, _ := connect.CallInfoForClientContext(ctx)
			callInfo.RequestHeader().Set("Authorization", authorization)
			return next(ctx, spec)
		}
	}
}

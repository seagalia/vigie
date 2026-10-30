package clients_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/sdk/clients"
	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
	"github.com/seagalia/vigie/sdk/vigie/eds/v1/edsconnect"
)

// fakeEDS answers one endpoint and keeps the Authorization header of the last call.
type fakeEDS struct {
	authorization string
}

func (f *fakeEDS) GetEndpoints(ctx context.Context, _ *eds.GetEndpointsRequest) (*eds.GetEndpointsResponse, error) {
	callInfo, _ := connect.CallInfoForServerContext(ctx)
	f.authorization = callInfo.RequestHeader().Get("Authorization")
	endpoint := &eds.Endpoint{Ip: "10.0.0.1", Port: 8080, LoadBalancingWeight: 1}
	return &eds.GetEndpointsResponse{Endpoints: []*eds.Endpoint{endpoint}}, nil
}

func edsMux(fake *fakeEDS) *http.ServeMux {
	server := connect.NewServer()
	edsconnect.RegisterEndpointDiscoveryServiceHandler(server, fake)
	mux := http.NewServeMux()
	connecthttp.Mount(mux, server)
	return mux
}

func startH2CServer(handler http.Handler) *httptest.Server {
	srv := httptest.NewUnstartedServer(handler)
	srv.Config.Protocols = &http.Protocols{}
	srv.Config.Protocols.SetHTTP1(true)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	DeferCleanup(srv.Close)
	return srv
}

func getEndpoints(baseURL string, opts clients.Options) (*eds.GetEndpointsResponse, error) {
	client, err := clients.NewEndpointDiscovery(baseURL, opts)
	Expect(err).ToNot(HaveOccurred())
	return client.GetEndpoints(context.Background(), &eds.GetEndpointsRequest{ServiceName: "web"})
}

var _ = Describe("NewEndpointDiscovery", func() {
	var fake *fakeEDS

	BeforeEach(func() {
		fake = &fakeEDS{}
	})

	It("calls over h2c", func() {
		srv := startH2CServer(edsMux(fake))

		resp, err := getEndpoints(srv.URL, clients.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.GetEndpoints()).To(HaveLen(1))
		Expect(resp.GetEndpoints()[0].GetIp()).To(Equal("10.0.0.1"))
	})

	It("calls over tls", func() {
		srv := httptest.NewUnstartedServer(edsMux(fake))
		srv.EnableHTTP2 = true
		srv.StartTLS()
		DeferCleanup(srv.Close)
		roots := x509.NewCertPool()
		roots.AddCert(srv.Certificate())

		resp, err := getEndpoints(srv.URL, clients.Options{TLSConfig: &tls.Config{RootCAs: roots}})
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.GetEndpoints()).To(HaveLen(1))
	})

	It("sends basic auth", func() {
		srv := startH2CServer(edsMux(fake))

		_, err := getEndpoints(srv.URL, clients.Options{Username: "flock", Password: "secret"})
		Expect(err).ToNot(HaveOccurred())
		Expect(fake.authorization).To(Equal("Basic " + base64.StdEncoding.EncodeToString([]byte("flock:secret"))))

		_, err = getEndpoints(srv.URL, clients.Options{})
		Expect(err).ToNot(HaveOccurred())
		Expect(fake.authorization).To(BeEmpty())
	})

	It("puts the path before the procedure", func() {
		mux := http.NewServeMux()
		mux.Handle("/vigie/", http.StripPrefix("/vigie", edsMux(fake)))
		srv := startH2CServer(mux)

		_, err := getEndpoints(srv.URL+"/vigie", clients.Options{})
		Expect(err).ToNot(HaveOccurred())

		_, err = getEndpoints(srv.URL+"/services", clients.Options{})
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeUnimplemented))
	})

	It("refuses another scheme", func() {
		_, err := clients.NewEndpointDiscovery("grpc://x:1", clients.Options{})
		Expect(err).To(MatchError(ContainSubstring(`"grpc"`)))
	})
})

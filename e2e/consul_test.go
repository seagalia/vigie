package e2e_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/config"
	"github.com/seagalia/vigie/services"
	"github.com/testcontainers/testcontainers-go/modules/consul"
)

var _ = Describe("Consul through the dns source", Label("e2e"), Ordered, func() {
	var resolver string

	BeforeAll(func(ctx context.Context) {
		// by default Consul DNS also gives the instances in warning, the documented setup leaves them out
		container, err := consul.Run(ctx, "hashicorp/consul:2.1", consul.WithConfigString(`{"dns_config": {"only_passing": true}}`))
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func(ctx context.Context) {
			Expect(container.Terminate(ctx)).To(Succeed())
		})
		apiEndpoint, err := container.ApiEndpoint(ctx)
		Expect(err).ToNot(HaveOccurred())
		host, err := container.Host(ctx)
		Expect(err).ToNot(HaveOccurred())
		dnsPort, err := container.MappedPort(ctx, "8600/tcp")
		Expect(err).ToNot(HaveOccurred())
		resolver = net.JoinHostPort(host, dnsPort.Port())

		registerConsul(apiEndpoint, `{"ID": "api-1", "Name": "api", "Address": "10.0.0.1", "Port": 9000, "Tags": ["primary"], "Check": {"TTL": "10m", "Status": "passing"}}`)
		registerConsul(apiEndpoint, `{"ID": "api-2", "Name": "api", "Address": "10.0.0.2", "Port": 9000, "Check": {"TTL": "10m", "Status": "critical"}}`)
		registerConsul(apiEndpoint, `{"ID": "api-3", "Name": "api", "Address": "10.0.0.3", "Port": 9001, "Weights": {"Passing": 5, "Warning": 1}, "Check": {"TTL": "10m", "Status": "passing"}}`)
	}, NodeTimeout(3*time.Minute))

	consulHandler := func(srvProto string) *services.Handler {
		handler, err := services.NewHandler([]*config.Service{{
			Name:        "api",
			DefaultPort: 8080,
			DNS: &config.ServiceDNS{
				Name:       "service.consul",
				Type:       "SRV",
				SrvService: "api",
				SrvProto:   srvProto,
				Resolver:   resolver,
			},
		}})
		Expect(err).ToNot(HaveOccurred())
		return handler
	}

	It("gives the passing instances with their port and weight", func() {
		handler := consulHandler("tcp")

		// Consul serves a registration once its catalog has it, a moment after the register call
		Eventually(func() []endpoint {
			_, ips := getServices(handler, "api")
			return ips
		}).WithTimeout(30 * time.Second).Should(ConsistOf(
			endpoint{IP: "10.0.0.1", Port: 9000, LoadBalancingWeight: 1},
			endpoint{IP: "10.0.0.3", Port: 9001, LoadBalancingWeight: 5},
		))
	})

	It("keeps the instances with the tag given in place of tcp", func() {
		handler := consulHandler("primary")

		Eventually(func() []endpoint {
			_, ips := getServices(handler, "api")
			return ips
		}).WithTimeout(30 * time.Second).Should(ConsistOf(
			endpoint{IP: "10.0.0.1", Port: 9000, LoadBalancingWeight: 1},
		))
	})
})

// registerConsul registers a service on the agent with the HTTP API, the test needs no Consul library for one call
func registerConsul(apiEndpoint, definition string) {
	req, err := http.NewRequest(http.MethodPut, "http://"+apiEndpoint+"/v1/agent/service/register", bytes.NewBufferString(definition))
	Expect(err).ToNot(HaveOccurred())
	resp, err := http.DefaultClient.Do(req)
	Expect(err).ToNot(HaveOccurred())
	defer resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusOK))
}

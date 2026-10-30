package ui

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/config"
	"go.yaml.in/yaml/v2"
)

var _ = Describe("describeSource", func() {
	It("gives the srv name a dns srv service resolves", func() {
		svc := &config.Service{DNS: &config.ServiceDNS{Name: "web.local", Type: "SRV", SrvService: "http", SrvProto: "tcp"}}

		kind, source := describeSource(svc)

		Expect(kind).To(Equal("dns srv"))
		Expect(source).To(Equal("_http._tcp.web.local"))
	})

	It("shows the resolver of a dns source", func() {
		svc := &config.Service{DNS: &config.ServiceDNS{Name: "service.consul", Type: "SRV", SrvService: "api", SrvProto: "tcp", Resolver: "127.0.0.1:8600"}}

		kind, source := describeSource(svc)

		Expect(kind).To(Equal("dns srv"))
		Expect(source).To(Equal("_api._tcp.service.consul @127.0.0.1:8600"))
	})

	It("describes a kubernetes source without its kubeconfig", func() {
		svc := &config.Service{Kubernetes: &config.ServiceKubernetes{Namespace: "shop", Service: "web", PortName: "http", Kubeconfig: "/secret/kubeconfig"}}

		kind, source := describeSource(svc)

		Expect(kind).To(Equal("kubernetes"))
		Expect(source).To(Equal("shop/web:http"))
	})

	It("describes a kubernetes source without port name", func() {
		_, source := describeSource(&config.Service{Kubernetes: &config.ServiceKubernetes{Namespace: "shop", Service: "web"}})

		Expect(source).To(Equal("shop/web"))
	})

	It("never shows the credentials of an url", func() {
		svc := &config.Service{URL: &config.ServiceURL{}}
		Expect(yaml.Unmarshal([]byte("url: https://user:secret@eds.local/web\n"), svc.URL)).To(Succeed())

		kind, source := describeSource(svc)

		Expect(kind).To(Equal("url"))
		Expect(source).To(Equal("https://eds.local/web"))
	})
})

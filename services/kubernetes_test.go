package services_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/config"
	. "github.com/seagalia/vigie/services"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

var _ = Describe("Kubernetes", func() {
	webService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "web"}}

	port := func(name string, number int32) discoveryv1.EndpointPort {
		return discoveryv1.EndpointPort{Name: ptr.To(name), Port: ptr.To(number)}
	}
	// a nil ready leaves the condition unset, which Kubernetes reads as ready
	endpoint := func(ready *bool, addresses ...string) discoveryv1.Endpoint {
		return discoveryv1.Endpoint{Addresses: addresses, Conditions: discoveryv1.EndpointConditions{Ready: ready, Serving: ptr.To(true)}}
	}
	slice := func(name, service string, addressType discoveryv1.AddressType, ports []discoveryv1.EndpointPort, endpoints ...discoveryv1.Endpoint) *discoveryv1.EndpointSlice {
		return &discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      name,
				Labels:    map[string]string{discoveryv1.LabelServiceName: service},
			},
			AddressType: addressType,
			Ports:       ports,
			Endpoints:   endpoints,
		}
	}
	resolve := func(portName string, objects ...runtime.Object) ([]IpAddr, error) {
		client := fake.NewClientset(objects...)
		cnf := &config.ServiceKubernetes{Namespace: "shop", Service: "web", PortName: portName}
		return NewKubernetesWithClient(client, cnf, 8080).Resolve()
	}
	http := []discoveryv1.EndpointPort{port("http", 80)}

	It("keeps ready and unset-ready endpoints and skips the ones not ready", func() {
		ips, err := resolve("", webService, slice("web-1", "web", discoveryv1.AddressTypeIPv4, http,
			endpoint(ptr.To(true), "10.0.0.1"),
			endpoint(nil, "10.0.0.2"),
			endpoint(ptr.To(false), "10.0.0.3"),
		))

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(Equal([]IpAddr{{IP: "10.0.0.1", Port: 80}, {IP: "10.0.0.2", Port: 80}}))
	})

	It("gives every address of an endpoint", func() {
		ips, err := resolve("", webService, slice("web-1", "web", discoveryv1.AddressTypeIPv4, http,
			endpoint(ptr.To(true), "10.0.0.1", "10.0.0.4"),
		))

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(Equal([]IpAddr{{IP: "10.0.0.1", Port: 80}, {IP: "10.0.0.4", Port: 80}}))
	})

	It("reads every slice of the service, ipv4 and ipv6, and skips fqdn", func() {
		ips, err := resolve("", webService,
			slice("web-v4", "web", discoveryv1.AddressTypeIPv4, http, endpoint(ptr.To(true), "10.0.0.1")),
			slice("web-v6", "web", discoveryv1.AddressTypeIPv6, http, endpoint(ptr.To(true), "fd00::1")),
			slice("web-fqdn", "web", discoveryv1.AddressTypeFQDN, http, endpoint(ptr.To(true), "web.example.com")),
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(ConsistOf(IpAddr{IP: "10.0.0.1", Port: 80}, IpAddr{IP: "fd00::1", Port: 80}))
	})

	It("ignores the slices of another service", func() {
		ips, err := resolve("", webService,
			slice("web-1", "web", discoveryv1.AddressTypeIPv4, http, endpoint(ptr.To(true), "10.0.0.1")),
			slice("other-1", "other", discoveryv1.AddressTypeIPv4, http, endpoint(ptr.To(true), "10.0.0.9")),
		)

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(Equal([]IpAddr{{IP: "10.0.0.1", Port: 80}}))
	})

	It("picks the named port", func() {
		ports := []discoveryv1.EndpointPort{port("metrics", 9090), port("http", 80)}

		ips, err := resolve("http", webService, slice("web-1", "web", discoveryv1.AddressTypeIPv4, ports, endpoint(ptr.To(true), "10.0.0.1")))

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(Equal([]IpAddr{{IP: "10.0.0.1", Port: 80}}))
	})

	It("takes the only port without a port name", func() {
		ips, err := resolve("", webService, slice("web-1", "web", discoveryv1.AddressTypeIPv4, http, endpoint(ptr.To(true), "10.0.0.1")))

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(Equal([]IpAddr{{IP: "10.0.0.1", Port: 80}}))
	})

	It("falls back to the default port with several ports and no port name", func() {
		ports := []discoveryv1.EndpointPort{port("metrics", 9090), port("http", 80)}

		ips, err := resolve("", webService, slice("web-1", "web", discoveryv1.AddressTypeIPv4, ports, endpoint(ptr.To(true), "10.0.0.1")))

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(Equal([]IpAddr{{IP: "10.0.0.1", Port: 8080}}))
	})

	It("falls back to the default port when the port has no name and no number", func() {
		ports := []discoveryv1.EndpointPort{{}}

		ips, err := resolve("", webService, slice("web-1", "web", discoveryv1.AddressTypeIPv4, ports, endpoint(ptr.To(true), "10.0.0.1")))

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(Equal([]IpAddr{{IP: "10.0.0.1", Port: 8080}}))
	})

	It("fails on a port name the slice doesn't have", func() {
		_, err := resolve("grpc", webService, slice("web-1", "web", discoveryv1.AddressTypeIPv4, http, endpoint(ptr.To(true), "10.0.0.1")))

		Expect(err).To(MatchError("port grpc not found in service shop/web"))
	})

	It("answers an empty list for a service without ready endpoint", func() {
		ips, err := resolve("", webService, slice("web-1", "web", discoveryv1.AddressTypeIPv4, http, endpoint(ptr.To(false), "10.0.0.1")))

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(BeEmpty())
		Expect(ips).ToNot(BeNil())
	})

	It("answers an empty list for the placeholder slice of a service scaled to zero", func() {
		placeholder := slice("web-1", "web", discoveryv1.AddressTypeIPv4, []discoveryv1.EndpointPort{})

		ips, err := resolve("http", webService, placeholder)

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(BeEmpty())
	})

	It("fails for a missing service", func() {
		_, err := resolve("", slice("web-1", "web", discoveryv1.AddressTypeIPv4, http, endpoint(ptr.To(true), "10.0.0.1")))

		Expect(err).To(MatchError(ContainSubstring("shop/web")))
	})
})

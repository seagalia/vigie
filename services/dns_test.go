package services_test

import (
	"net"
	"time"

	"github.com/miekg/dns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/config"
	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
	. "github.com/seagalia/vigie/services"
	"google.golang.org/protobuf/testing/protocmp"
)

// answerTestZone answers the few names the specs ask, and an empty answer for anything else
func answerTestZone(w dns.ResponseWriter, req *dns.Msg) {
	defer GinkgoRecover()
	resp := new(dns.Msg)
	resp.SetReply(req)
	question := req.Question[0]
	records := map[uint16]map[string][]string{
		dns.TypeA: {
			"web.test.":          {"web.test. 60 IN A 10.0.0.1"},
			"node1.node.consul.": {"node1.node.consul. 60 IN A 10.0.0.9"},
		},
		dns.TypeSRV: {
			"_api._tcp.service.consul.": {"_api._tcp.service.consul. 60 IN SRV 1 3 9000 node1.node.consul."},
		},
		dns.TypeTXT: {
			"eds.test.": {`eds.test. 60 IN TXT "eds=10.0.0.2:81#4"`, `eds.test. 60 IN TXT "other"`},
		},
	}
	for _, record := range records[question.Qtype][question.Name] {
		rr, err := dns.NewRR(record)
		Expect(err).ToNot(HaveOccurred())
		resp.Answer = append(resp.Answer, rr)
	}
	_ = w.WriteMsg(resp)
}

var _ = Describe("DNS with a resolver", func() {
	var addr string

	BeforeEach(func() {
		// tcp only, so a query sent over udp can't get an answer
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).ToNot(HaveOccurred())
		server := &dns.Server{Listener: listener, Handler: dns.HandlerFunc(answerTestZone)}
		go func() { _ = server.ActivateAndServe() }()
		DeferCleanup(server.Shutdown)
		addr = listener.Addr().String()
	})

	It("should look up ips through the resolver", func() {
		ips, err := NewDNS(&config.ServiceDNS{Name: "web.test", Type: "IP", Resolver: addr}, 80).Resolve()

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(BeComparableTo([]*eds.Endpoint{{Ip: "10.0.0.1", Port: 80}}, protocmp.Transform()))
	})

	It("should look up srv records and their targets through the resolver", func() {
		cnf := &config.ServiceDNS{Name: "service.consul", Type: "SRV", SrvService: "api", SrvProto: "tcp", Resolver: addr}

		ips, err := NewDNS(cnf, 80).Resolve()

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(BeComparableTo([]*eds.Endpoint{{Ip: "10.0.0.9", Port: 9000, LoadBalancingWeight: 3}}, protocmp.Transform()))
	})

	It("should look up txt records through the resolver", func() {
		ips, err := NewDNS(&config.ServiceDNS{Name: "eds.test", Type: "TXT", Resolver: addr}, 80).Resolve()

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(BeComparableTo([]*eds.Endpoint{{Ip: "10.0.0.2", Port: 81, LoadBalancingWeight: 4}}, protocmp.Transform()))
	})

	It("should fail without hanging when the resolver is unreachable", func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).ToNot(HaveOccurred())
		closed := listener.Addr().String()
		Expect(listener.Close()).To(Succeed())
		start := time.Now()

		_, err = NewDNS(&config.ServiceDNS{Name: "web.test", Type: "IP", Resolver: closed}, 80).Resolve()

		Expect(err).To(HaveOccurred())
		Expect(time.Since(start)).To(BeNumerically("<", 9*time.Second))
	})
})

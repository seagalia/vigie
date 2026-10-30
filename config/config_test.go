package config_test

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/seagalia/vigie/config"
)

var _ = Describe("Config", func() {
	write := func(content string) string {
		path := filepath.Join(GinkgoT().TempDir(), "config.eds.yml")
		Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
		return path
	}

	It("should give defaults to the server and the services", func() {
		cnf, err := ReadConfig(write("services:\n- name: web\n  dns: {name: web.local, type: srv}\n"))

		Expect(err).ToNot(HaveOccurred())
		Expect(cnf.ServerConfig.ListenAddr).To(Equal(":8080"))
		Expect(cnf.Services[0].DefaultPort).To(Equal(uint32(80)))
		Expect(cnf.Services[0].DNS.Type).To(Equal("SRV"))
		Expect(cnf.Services[0].DNS.SrvProto).To(Equal("tcp"))
		Expect(cnf.Services[0].DNS.SrvService).To(Equal("http"))
	})

	DescribeTable("should refuse invalid configurations",
		func(content, expected string) {
			_, err := ReadConfig(write(content))
			Expect(err).To(MatchError(ContainSubstring(expected)))
		},
		Entry("no service", "server: {listen_addr: ':9000'}\n", "services is empty"),
		Entry("service without source", "services:\n- name: web\n", "service dns, file, url and kubernetes are empty"),
		Entry("kubernetes without service", "services:\n- name: web\n  kubernetes: {namespace: shop}\n", "service kubernetes service is empty"),
		Entry("unknown dns type", "services:\n- name: web\n  dns: {name: web.local, type: mx}\n", "valid are IP, TXT, SRV"),
		Entry("resolver without port", "services:\n- name: web\n  dns: {name: web.local, resolver: 127.0.0.1}\n", "service dns resolver is invalid"),
		Entry("resolver without host", "services:\n- name: web\n  dns: {name: web.local, resolver: ':8600'}\n", "service dns resolver is invalid"),
		Entry("negative global cache ttl", "cache: {ttl: -1s}\nservices:\n- name: web\n  file: {path: x}\n", "cache ttl is negative"),
		Entry("negative service cache ttl", "services:\n- name: web\n  cache_ttl: -1s\n  file: {path: x}\n", "service web cache_ttl is negative"),
		Entry("tls without key", "server: {tls: {cert_file: a.pem}}\nservices:\n- name: web\n  file: {path: x}\n", "cert_file and key_file are required"),
	)

	It("should cache every service for 5 seconds by default", func() {
		cnf, err := ReadConfig(write("services:\n- name: web\n  file: {path: x}\n"))
		Expect(err).ToNot(HaveOccurred())
		Expect(*cnf.Services[0].CacheTTL).To(Equal(5 * time.Second))
	})

	It("should give the global cache ttl to the services without their own", func() {
		cnf, err := ReadConfig(write("cache: {ttl: 1m}\nservices:\n- name: web\n  file: {path: x}\n- name: api\n  cache_ttl: 0s\n  file: {path: x}\n"))
		Expect(err).ToNot(HaveOccurred())
		Expect(*cnf.Services[0].CacheTTL).To(Equal(time.Minute))
		Expect(*cnf.Services[1].CacheTTL).To(Equal(time.Duration(0)))
	})

	It("should give the default namespace to a kubernetes service", func() {
		cnf, err := ReadConfig(write("services:\n- name: web\n  kubernetes: {service: web}\n"))
		Expect(err).ToNot(HaveOccurred())
		Expect(cnf.Services[0].Kubernetes.Namespace).To(Equal("default"))
	})

	It("should accept an ipv6 resolver", func() {
		cnf, err := ReadConfig(write("services:\n- name: web\n  dns: {name: web.local, resolver: '[::1]:8600'}\n"))
		Expect(err).ToNot(HaveOccurred())
		Expect(cnf.Services[0].DNS.Resolver).To(Equal("[::1]:8600"))
	})

	It("should refuse unknown keys in strict mode only", func() {
		path := write("services:\n- name: web\n  file: {path: x}\nunknown: true\n")

		_, err := ReadConfig(path)
		Expect(err).ToNot(HaveOccurred())
		_, err = ReadConfigStrict(path)
		Expect(err).To(HaveOccurred())
	})
})

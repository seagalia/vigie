package services

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/config"
)

var _ = Describe("Kubernetes client config", func() {
	It("has no client-side rate limit, flock agents polling at once would wait past the request timeout", func() {
		kubeconfig := filepath.Join(GinkgoT().TempDir(), "kubeconfig")
		Expect(os.WriteFile(kubeconfig, []byte(`apiVersion: v1
kind: Config
clusters: [{name: test, cluster: {server: "https://127.0.0.1:6443"}}]
users: [{name: test, user: {token: secret}}]
contexts: [{name: test, context: {cluster: test, user: test}}]
current-context: test
`), 0600)).To(Succeed())

		restConfig, err := newRestConfig(&config.ServiceKubernetes{Namespace: "shop", Service: "web", Kubeconfig: kubeconfig})

		Expect(err).ToNot(HaveOccurred())
		Expect(restConfig.QPS).To(BeNumerically("<", 0))
	})
})

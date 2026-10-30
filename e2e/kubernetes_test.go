package e2e_test

import (
	"context"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/config"
	"github.com/seagalia/vigie/services"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/kind/pkg/cluster"
)

const kindCluster = "vigie-e2e"

var _ = Describe("Kubernetes source", Label("e2e"), Ordered, func() {
	var kubeconfig string

	// one cluster for every spec, starting one takes about a minute
	BeforeAll(func(ctx context.Context) {
		kubeconfig = filepath.Join(GinkgoT().TempDir(), "kubeconfig")
		provider := cluster.NewProvider()
		// a cluster left by an interrupted run would make Create fail
		_ = provider.Delete(kindCluster, "")
		err := provider.Create(kindCluster,
			cluster.CreateWithKubeconfigPath(kubeconfig),
			cluster.CreateWithWaitForReady(2*time.Minute),
		)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(provider.Delete(kindCluster, kubeconfig)).To(Succeed())
		})

		restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		Expect(err).ToNot(HaveOccurred())
		client, err := kubernetes.NewForConfig(restConfig)
		Expect(err).ToNot(HaveOccurred())
		createWebService(ctx, client)
	}, NodeTimeout(5*time.Minute))

	It("gives the ready endpoints of the service", func() {
		handler, err := services.NewHandler([]*config.Service{{
			Name:        "web",
			DefaultPort: 8080,
			Kubernetes: &config.ServiceKubernetes{
				Namespace:  "shop",
				Service:    "web",
				Kubeconfig: kubeconfig,
				Context:    "kind-" + kindCluster,
			},
		}})
		Expect(err).ToNot(HaveOccurred())

		code, ips := getServices(handler, "web")

		Expect(code).To(Equal(200))
		Expect(ips).To(Equal([]services.IpAddr{{IP: "10.0.0.1", Port: 80}}))
	})
})

// createWebService writes shop/web without selector, with an EndpointSlice written by hand, so no pod has to run
func createWebService(ctx context.Context, client kubernetes.Interface) {
	_, err := client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "shop"}}, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "web"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80}}},
	}
	_, err = client.CoreV1().Services("shop").Create(ctx, service, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	slice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "shop",
			Name:      "web-1",
			Labels:    map[string]string{discoveryv1.LabelServiceName: "web"},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Ports:       []discoveryv1.EndpointPort{{Name: ptr.To("http"), Port: ptr.To(int32(80))}},
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)}},
			{Addresses: []string{"10.0.0.2"}, Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(false)}},
		},
	}
	_, err = client.DiscoveryV1().EndpointSlices("shop").Create(ctx, slice, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
}

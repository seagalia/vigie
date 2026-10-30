package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/seagalia/vigie/config"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Kubernetes struct {
	client      kubernetes.Interface
	cnf         *config.ServiceKubernetes
	defaultPort uint32
}

// NewKubernetes builds the client once, so a kubeconfig that can't be loaded fails the load of the services, not
// every request
func NewKubernetes(cnf *config.ServiceKubernetes, defaultPort uint32) (*Kubernetes, error) {
	restConfig, err := newRestConfig(cnf)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}
	return NewKubernetesWithClient(client, cnf, defaultPort), nil
}

func NewKubernetesWithClient(client kubernetes.Interface, cnf *config.ServiceKubernetes, defaultPort uint32) *Kubernetes {
	return &Kubernetes{client: client, cnf: cnf, defaultPort: defaultPort}
}

func newRestConfig(cnf *config.ServiceKubernetes) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if cnf.Kubeconfig != "" {
		path, err := expandHome(cnf.Kubeconfig)
		if err != nil {
			return nil, err
		}
		rules = &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}
	}
	overrides := &clientcmd.ConfigOverrides{CurrentContext: cnf.Context}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubernetes client config: %w", err)
	}
	// every /services request makes two api calls, the default limit of 5 per second would queue the requests of a
	// few dozen flock agents past their timeout; a negative value turns the limiter off
	restConfig.QPS = -1
	return restConfig, nil
}

func expandHome(path string) (string, error) {
	rest, hasHome := strings.CutPrefix(path, "~/")
	if !hasHome {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand %s: %w", path, err)
	}
	return filepath.Join(home, rest), nil
}

func (s *Kubernetes) Resolve() ([]IpAddr, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ns, name := s.cnf.Namespace, s.cnf.Service

	// the slices of a missing service are an empty list too, the get tells a typo from a service with no endpoint
	_, err := s.client.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get service %s/%s: %w", ns, name, err)
	}
	slices, err := s.client.DiscoveryV1().EndpointSlices(ns).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + name,
	})
	if err != nil {
		return nil, fmt.Errorf("list endpointslices of service %s/%s: %w", ns, name, err)
	}

	ipAddrs := make([]IpAddr, 0)
	for _, slice := range slices.Items {
		// flock dials ips, a slice of host names gives it nothing to dial
		if slice.AddressType == discoveryv1.AddressTypeFQDN {
			continue
		}
		addresses := readyAddresses(slice.Endpoints)
		// a service scaled to zero keeps a placeholder slice without ports, its port name can't be found there
		if len(addresses) == 0 {
			continue
		}
		port, err := s.slicePort(slice.Ports)
		if err != nil {
			return nil, err
		}
		for _, address := range addresses {
			ipAddrs = append(ipAddrs, IpAddr{IP: address, Port: port})
		}
	}
	return ipAddrs, nil
}

func readyAddresses(endpoints []discoveryv1.Endpoint) []string {
	addresses := make([]string, 0)
	for _, endpoint := range endpoints {
		isReady := endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready
		if isReady {
			addresses = append(addresses, endpoint.Addresses...)
		}
	}
	return addresses
}

// slicePort gives the port named port_name, the only port of the slice without port_name, else default_port
func (s *Kubernetes) slicePort(ports []discoveryv1.EndpointPort) (uint32, error) {
	if s.cnf.PortName == "" {
		if len(ports) == 1 && ports[0].Port != nil {
			return uint32(*ports[0].Port), nil
		}
		return s.defaultPort, nil
	}
	for _, port := range ports {
		isNamed := port.Name != nil && *port.Name == s.cnf.PortName
		if isNamed && port.Port != nil {
			return uint32(*port.Port), nil
		}
	}
	return 0, fmt.Errorf("port %s not found in service %s/%s", s.cnf.PortName, s.cnf.Namespace, s.cnf.Service)
}

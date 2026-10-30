package services

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/seagalia/vigie/config"
)

type Handler struct {
	// a reload replaces the services while requests read them
	svcs atomic.Pointer[map[string]Service]
}

func NewHandler(svcsConfig []*config.Service) (*Handler, error) {
	h := &Handler{}
	err := h.LoadServices(svcsConfig)
	if err != nil {
		return nil, err
	}
	return h, nil
}

// LoadServices replaces all the services, or none of them when one can't be built
func (h *Handler) LoadServices(svcsConfig []*config.Service) error {
	svcs := make(map[string]Service)
	for _, svcConfig := range svcsConfig {
		svc, err := newSource(svcConfig)
		if err != nil {
			return fmt.Errorf("service %s: %w", svcConfig.Name, err)
		}
		hasCache := svcConfig.CacheTTL != nil && *svcConfig.CacheTTL > 0
		if hasCache {
			svc = NewCache(svc, *svcConfig.CacheTTL)
		}
		svcs[svcConfig.Name] = svc
	}
	h.svcs.Store(&svcs)
	return nil
}

// newSource builds the one source of a service; when several are set, the first in this order wins
func newSource(svcConfig *config.Service) (Service, error) {
	if svcConfig.Kubernetes != nil {
		return NewKubernetes(svcConfig.Kubernetes, svcConfig.DefaultPort)
	}
	if svcConfig.URL != nil {
		return NewURL(svcConfig.URL, svcConfig.DefaultPort), nil
	}
	if svcConfig.File != nil {
		return NewFile(svcConfig.File, svcConfig.DefaultPort), nil
	}
	if svcConfig.DNS != nil {
		return NewDNS(svcConfig.DNS, svcConfig.DefaultPort), nil
	}
	return nil, fmt.Errorf("no source")
}

// lookup finds a service of the loaded ones, the one named default for an empty name
func (h *Handler) lookup(name string) (string, Service, bool) {
	if name == "" {
		name = "default"
	}
	svc, ok := (*h.svcs.Load())[name]
	return name, svc, ok
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	svc, s, ok := h.lookup(r.URL.Query().Get("service_name"))
	if !ok {
		http.Error(w, fmt.Sprintf("service %s not found", svc), http.StatusNotFound)
		return
	}

	endpoints, err := s.Resolve()
	if err != nil {
		http.Error(w, fmt.Sprintf("resolve service %s failed: %s", svc, err), http.StatusInternalServerError)
		return
	}
	body, err := marshalEndpointsJSON(endpoints)
	if err != nil {
		http.Error(w, fmt.Sprintf("encode endpoints of service %s failed: %s", svc, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

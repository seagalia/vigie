package e2e_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/services"
)

// endpoint is an object of the answer of /services as flock and meridian decode it, apart from vigie's own types
type endpoint struct {
	IP                  string `json:"ip"`
	Port                uint32 `json:"port"`
	LoadBalancingWeight uint32 `json:"load_balancing_weight"`
}

// getServices asks the handler the way flock and meridian do, and decodes its answer when it is a 200
func getServices(handler *services.Handler, name string) (int, []endpoint) {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/services?service_name="+name, nil))
	if recorder.Code != http.StatusOK {
		return recorder.Code, nil
	}
	var ips []endpoint
	Expect(json.Unmarshal(recorder.Body.Bytes(), &ips)).To(Succeed())
	return recorder.Code, ips
}

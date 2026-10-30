package e2e_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/services"
)

// getServices asks the handler the way flock and meridian do, and decodes its answer when it is a 200
func getServices(handler *services.Handler, name string) (int, []services.IpAddr) {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/services?service_name="+name, nil))
	if recorder.Code != http.StatusOK {
		return recorder.Code, nil
	}
	var ips []services.IpAddr
	Expect(json.Unmarshal(recorder.Body.Bytes(), &ips)).To(Succeed())
	return recorder.Code, ips
}

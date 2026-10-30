package services_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/config"
	. "github.com/seagalia/vigie/services"
	"go.yaml.in/yaml/v2"
)

var _ = Describe("Services", func() {
	writeEndpoints := func(content string) string {
		path := filepath.Join(GinkgoT().TempDir(), "endpoints.txt")
		Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
		return path
	}

	It("should read endpoints of a file with their port and weight", func() {
		path := writeEndpoints("# backends\n10.0.0.1\n\n10.0.0.2:8080#5\nhttp://10.0.0.3:9090\n")

		ips, err := NewFile(&config.ServiceFile{Path: path}, 80).Resolve()

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(Equal([]IpAddr{
			{IP: "10.0.0.1", Port: 80},
			{IP: "10.0.0.2", Port: 8080, LoadBalancingWeight: 5},
			{IP: "10.0.0.3", Port: 9090},
		}))
	})

	It("should refuse an invalid weight", func() {
		_, err := NewFile(&config.ServiceFile{Path: writeEndpoints("10.0.0.1#heavy\n")}, 80).Resolve()
		Expect(err).To(MatchError(ContainSubstring("invalid weight")))
	})

	It("should read endpoints of an url with basic auth", func() {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, password, ok := r.BasicAuth()
			if !ok || user != "user" || password != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte("10.0.0.1:81\n"))
		}))
		DeferCleanup(backend.Close)
		urlConf := &config.ServiceURL{}
		Expect(yaml.Unmarshal([]byte("url: "+backend.URL+"\nbasic_auth: {username: user, password: secret}\n"), urlConf)).To(Succeed())

		ips, err := NewURL(urlConf, 80).Resolve()

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(Equal([]IpAddr{{IP: "10.0.0.1", Port: 81}}))
	})

	It("should give the error of the url", func() {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("upstream down"))
		}))
		DeferCleanup(backend.Close)
		urlConf := &config.ServiceURL{}
		Expect(yaml.Unmarshal([]byte("url: "+backend.URL+"\n"), urlConf)).To(Succeed())

		_, err := NewURL(urlConf, 80).Resolve()

		Expect(err).To(MatchError("bad status code: 502, upstream down"))
	})

	Context("Handler", func() {
		var handler *Handler
		BeforeEach(func() {
			var err error
			handler, err = NewHandler([]*config.Service{
				{Name: "default", DefaultPort: 80, File: &config.ServiceFile{Path: writeEndpoints("10.0.0.1\n")}},
			})
			Expect(err).ToNot(HaveOccurred())
		})

		get := func(query string) *httptest.ResponseRecorder {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/services"+query, nil))
			return recorder
		}

		It("should give the endpoints of the default service", func() {
			resp := get("")
			Expect(resp.Code).To(Equal(http.StatusOK))
			var ips []map[string]any
			Expect(json.Unmarshal(resp.Body.Bytes(), &ips)).To(Succeed())
			Expect(ips).To(Equal([]map[string]any{{"ip": "10.0.0.1", "port": float64(80), "load_balancing_weight": float64(0)}}))
		})

		It("should use the url when a service sets an url and a file", func() {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("10.0.0.7\n"))
			}))
			DeferCleanup(backend.Close)
			urlConf := &config.ServiceURL{}
			Expect(yaml.Unmarshal([]byte("url: "+backend.URL+"\n"), urlConf)).To(Succeed())
			file := &config.ServiceFile{Path: writeEndpoints("10.0.0.1\n")}
			var err error
			handler, err = NewHandler([]*config.Service{{Name: "default", DefaultPort: 80, URL: urlConf, File: file}})
			Expect(err).ToNot(HaveOccurred())

			Expect(get("").Body.String()).To(And(ContainSubstring(`"10.0.0.7"`), Not(ContainSubstring(`"10.0.0.1"`))))
		})

		It("should answer from the cache of a service during its cache ttl", func() {
			path := writeEndpoints("10.0.0.1\n")
			ttl := time.Minute
			var err error
			handler, err = NewHandler([]*config.Service{{Name: "default", DefaultPort: 80, CacheTTL: &ttl, File: &config.ServiceFile{Path: path}}})
			Expect(err).ToNot(HaveOccurred())
			Expect(get("").Body.String()).To(ContainSubstring(`"10.0.0.1"`))

			Expect(os.WriteFile(path, []byte("10.0.0.2\n"), 0600)).To(Succeed())

			Expect(get("").Body.String()).To(ContainSubstring(`"10.0.0.1"`))
		})

		It("should read the source on every request with a zero cache ttl", func() {
			path := writeEndpoints("10.0.0.1\n")
			ttl := time.Duration(0)
			var err error
			handler, err = NewHandler([]*config.Service{{Name: "default", DefaultPort: 80, CacheTTL: &ttl, File: &config.ServiceFile{Path: path}}})
			Expect(err).ToNot(HaveOccurred())
			Expect(get("").Body.String()).To(ContainSubstring(`"10.0.0.1"`))

			Expect(os.WriteFile(path, []byte("10.0.0.2\n"), 0600)).To(Succeed())

			Expect(get("").Body.String()).To(ContainSubstring(`"10.0.0.2"`))
		})

		It("should answer not found for an unknown service", func() {
			Expect(get("?service_name=other").Code).To(Equal(http.StatusNotFound))
		})

		It("should answer while services are reloaded", func() {
			wg := sync.WaitGroup{}
			for range 20 {
				wg.Add(2)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					err := handler.LoadServices([]*config.Service{
						{Name: "default", DefaultPort: 80, File: &config.ServiceFile{Path: writeEndpoints("10.0.0.2\n")}},
					})
					Expect(err).ToNot(HaveOccurred())
				}()
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					Expect(get("").Code).To(Equal(http.StatusOK))
				}()
			}
			wg.Wait()
		})
	})
})

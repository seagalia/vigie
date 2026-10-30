package app_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/seagalia/vigie/app"
	"github.com/seagalia/vigie/config"
)

var _ = Describe("App", func() {
	var dir string

	writeFile := func(name, content string) string {
		path := filepath.Join(dir, name)
		Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
		return path
	}

	// serve starts the handler of an app reading cnfContent from config.yml in dir
	serve := func(cnfContent string) *httptest.Server {
		cnfPath := writeFile("config.yml", cnfContent)
		cnf, err := config.ReadConfig(cnfPath)
		Expect(err).ToNot(HaveOccurred())
		a, err := NewApp(cnf, cnfPath, "1.2.3")
		Expect(err).ToNot(HaveOccurred())
		handler, _, err := a.Handler()
		Expect(err).ToNot(HaveOccurred())
		server := httptest.NewServer(handler)
		DeferCleanup(server.Close)
		return server
	}

	get := func(url string) (int, string) {
		resp, err := http.Get(url)
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		return resp.StatusCode, string(body)
	}

	post := func(url string) int {
		resp, err := http.Post(url, "text/plain", nil)
		Expect(err).ToNot(HaveOccurred())
		resp.Body.Close()
		return resp.StatusCode
	}

	configOf := func(endpointsPath string, extra string) string {
		return "services:\n- name: web\n  file: {path: " + endpointsPath + "}\n" + extra
	}

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
	})

	It("gives the endpoints of a service", func() {
		server := serve(configOf(writeFile("web.txt", "10.0.0.1\n"), ""))

		code, body := get(server.URL + "/services?service_name=web")

		Expect(code).To(Equal(http.StatusOK))
		Expect(body).To(ContainSubstring(`"ip": "10.0.0.1"`))
	})

	It("keeps the previous services when the new kubeconfig can't be loaded", func() {
		server := serve(configOf(writeFile("web.txt", "10.0.0.1\n"), ""))
		writeFile("config.yml", "services:\n- name: web\n  kubernetes: {service: web, kubeconfig: "+filepath.Join(dir, "missing")+"}\n")

		resp, err := http.Post(server.URL+"/reload", "text/plain", nil)
		Expect(err).ToNot(HaveOccurred())
		reloadBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusUnprocessableEntity))
		Expect(string(reloadBody)).To(ContainSubstring("load kubernetes client config"))
		code, body := get(server.URL + "/services?service_name=web")
		Expect(code).To(Equal(http.StatusOK))
		Expect(body).To(ContainSubstring(`"ip": "10.0.0.1"`))
	})

	It("answers the health check", func() {
		server := serve(configOf(writeFile("web.txt", "10.0.0.1\n"), ""))

		code, _ := get(server.URL + "/healthz")

		Expect(code).To(Equal(http.StatusOK))
	})

	It("lists the services on the page", func() {
		server := serve(configOf(writeFile("web.txt", "10.0.0.1\n"), ""))

		code, body := get(server.URL + "/")

		Expect(code).To(Equal(http.StatusOK))
		Expect(body).To(ContainSubstring(`data-service="web"`))
		Expect(body).To(ContainSubstring("vigie 1.2.3"))
		code, _ = get(server.URL + "/brand/lockup.svg")
		Expect(code).To(Equal(http.StatusOK))
		code, _ = get(server.URL + "/unknown")
		Expect(code).To(Equal(http.StatusNotFound))
	})

	It("reloads the services of the config file on POST /reload", func() {
		server := serve(configOf(writeFile("web.txt", "10.0.0.1\n"), ""))
		writeFile("config.yml", configOf(writeFile("web2.txt", "10.0.0.2\n"), ""))

		Expect(post(server.URL + "/reload")).To(Equal(http.StatusOK))

		_, body := get(server.URL + "/services?service_name=web")
		Expect(body).To(ContainSubstring(`"ip": "10.0.0.2"`))
	})

	It("keeps the previous services when the new config is invalid", func() {
		server := serve(configOf(writeFile("web.txt", "10.0.0.1\n"), ""))
		writeFile("config.yml", "services: []\n")

		Expect(post(server.URL + "/reload")).To(Equal(http.StatusUnprocessableEntity))

		_, body := get(server.URL + "/services?service_name=web")
		Expect(body).To(ContainSubstring(`"ip": "10.0.0.1"`))
	})

	Context("with basic authentication", func() {
		var server *httptest.Server

		BeforeEach(func() {
			server = serve(configOf(writeFile("web.txt", "10.0.0.1\n"), "server: {basic_auth: [{username: flock, password: secret}, {username: meridian, password: other}]}\n"))
		})

		send := func(method, path, username, password string) *http.Response {
			req, err := http.NewRequest(method, server.URL+path, nil)
			Expect(err).ToNot(HaveOccurred())
			if username != "" {
				req.SetBasicAuth(username, password)
			}
			resp, err := http.DefaultClient.Do(req)
			Expect(err).ToNot(HaveOccurred())
			resp.Body.Close()
			return resp
		}

		It("asks a request without credentials to authenticate", func() {
			resp := send(http.MethodGet, "/services?service_name=web", "", "")

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			Expect(resp.Header.Get("WWW-Authenticate")).To(Equal(`Basic realm="vigie", charset="UTF-8"`))
		})

		It("refuses a wrong password, and a password of another user", func() {
			Expect(send(http.MethodGet, "/services?service_name=web", "flock", "wrong").StatusCode).To(Equal(http.StatusUnauthorized))
			Expect(send(http.MethodGet, "/services?service_name=web", "flock", "other").StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("serves every user its endpoints, the page and the reload", func() {
			Expect(send(http.MethodGet, "/services?service_name=web", "flock", "secret").StatusCode).To(Equal(http.StatusOK))
			Expect(send(http.MethodGet, "/services?service_name=web", "meridian", "other").StatusCode).To(Equal(http.StatusOK))
			Expect(send(http.MethodGet, "/", "flock", "secret").StatusCode).To(Equal(http.StatusOK))
			Expect(send(http.MethodPost, "/reload", "flock", "secret").StatusCode).To(Equal(http.StatusOK))
		})

		It("protects the page and the reload", func() {
			Expect(send(http.MethodGet, "/", "", "").StatusCode).To(Equal(http.StatusUnauthorized))
			Expect(send(http.MethodGet, "/brand/lockup.svg", "", "").StatusCode).To(Equal(http.StatusUnauthorized))
			Expect(send(http.MethodPost, "/reload", "", "").StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("leaves the health check open for the probes", func() {
			Expect(send(http.MethodGet, "/healthz", "", "").StatusCode).To(Equal(http.StatusOK))
		})
	})

	It("refuses reload by request when disabled", func() {
		server := serve(configOf(writeFile("web.txt", "10.0.0.1\n"), "disable_reload_by_request: true\n"))

		Expect(post(server.URL + "/reload")).To(Equal(http.StatusNotFound))
	})
})

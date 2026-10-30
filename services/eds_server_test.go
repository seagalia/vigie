package services_test

import (
	"context"
	"os"
	"path/filepath"

	"connectrpc.com/connect/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/seagalia/vigie/config"
	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
	. "github.com/seagalia/vigie/services"
	"google.golang.org/protobuf/proto"
)

var _ = Describe("GetEndpoints", func() {
	var handler *Handler

	BeforeEach(func() {
		path := filepath.Join(GinkgoT().TempDir(), "endpoints.txt")
		Expect(os.WriteFile(path, []byte("10.0.0.1:8080\n"), 0600)).To(Succeed())
		var err error
		handler, err = NewHandler([]*config.Service{
			{Name: "web", DefaultPort: 80, File: &config.ServiceFile{Path: path}},
			{Name: "default", DefaultPort: 80, File: &config.ServiceFile{Path: path}},
			{Name: "broken", DefaultPort: 80, File: &config.ServiceFile{Path: filepath.Join(GinkgoT().TempDir(), "missing.txt")}},
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("answers the endpoints of a service", func() {
		resp, err := handler.GetEndpoints(context.Background(), &eds.GetEndpointsRequest{ServiceName: "web"})

		Expect(err).ToNot(HaveOccurred())
		Expect(resp.GetEndpoints()).To(HaveLen(1))
		Expect(proto.Equal(resp.GetEndpoints()[0], &eds.Endpoint{Ip: "10.0.0.1", Port: 8080})).To(BeTrue())
	})

	It("answers the service named default for an empty name", func() {
		resp, err := handler.GetEndpoints(context.Background(), &eds.GetEndpointsRequest{})

		Expect(err).ToNot(HaveOccurred())
		Expect(resp.GetEndpoints()).To(HaveLen(1))
	})

	It("answers NotFound for an unknown service", func() {
		_, err := handler.GetEndpoints(context.Background(), &eds.GetEndpointsRequest{ServiceName: "nope"})

		Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
		Expect(err.Error()).To(ContainSubstring("service nope not found"))
	})

	It("answers Unavailable when the source fails", func() {
		_, err := handler.GetEndpoints(context.Background(), &eds.GetEndpointsRequest{ServiceName: "broken"})

		Expect(connect.CodeOf(err)).To(Equal(connect.CodeUnavailable))
		Expect(err.Error()).To(ContainSubstring("resolve service broken failed"))
	})
})

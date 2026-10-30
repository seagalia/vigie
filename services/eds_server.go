package services

import (
	"context"

	"connectrpc.com/connect/v2"
	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
)

// GetEndpoints answers the EDS service with the same services and errors as GET /services.
func (h *Handler) GetEndpoints(_ context.Context, req *eds.GetEndpointsRequest) (*eds.GetEndpointsResponse, error) {
	name, svc, ok := h.lookup(req.GetServiceName())
	if !ok {
		return nil, connect.Errorf(connect.CodeNotFound, "service %s not found", name)
	}
	endpoints, err := svc.Resolve()
	if err != nil {
		return nil, connect.Errorf(connect.CodeUnavailable, "resolve service %s failed: %s", name, err)
	}
	return &eds.GetEndpointsResponse{Endpoints: endpoints}, nil
}

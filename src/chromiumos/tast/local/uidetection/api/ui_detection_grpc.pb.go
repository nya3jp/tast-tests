// package api provides the client API for UI detection service.
package api

import (
	context "context"

	grpc "google.golang.org/grpc"
)

type UiDetectionServiceClient struct {
	cc *grpc.ClientConn
}

// NewUiDetectionServiceClient creates a UiDetectionServiceClient instance.
func NewUiDetectionServiceClient(cc *grpc.ClientConn) *UiDetectionServiceClient {
	return &UiDetectionServiceClient{cc}
}

// Detect sends UI detection request to UiDetectionService.Detect in the server.
func (c *UiDetectionServiceClient) Detect(ctx context.Context, in *UiDetectionRequest, opts ...grpc.CallOption) (*UiDetectionResponse, error) {
	out := new(UiDetectionResponse)
	err := c.cc.Invoke(ctx, "/chromeos.acuiti.UiDetectionService/Detect", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

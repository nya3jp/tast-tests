// package api provides the client API for UI detection service.
package api

import (
	context "context"

	grpc "google.golang.org/grpc"
)

// UiDetectionServiceClient is the client API for UiDetectionService service.
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
// TODO(b/203185927): This file can be automatically generated if grpc > 1.30.
type UiDetectionServiceClient interface {
	// Runs the detection.
	Detect(ctx context.Context, in *UiDetectionRequest, opts ...grpc.CallOption) (*UiDetectionResponse, error)
}

type uiDetectionServiceClient struct {
	cc *grpc.ClientConn
}

// NewUiDetectionServiceClient create a
func NewUiDetectionServiceClient(cc *grpc.ClientConn) UiDetectionServiceClient {
	return &uiDetectionServiceClient{cc}
}

// Detect sends UI detection request to UiDetectionService.Detect in the server.
func (c *uiDetectionServiceClient) Detect(ctx context.Context, in *UiDetectionRequest, opts ...grpc.CallOption) (*UiDetectionResponse, error) {
	out := new(UiDetectionResponse)
	err := c.cc.Invoke(ctx, "/chromeos.acuiti.UiDetectionService/Detect", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UiDetectionServiceServer is the server API for UiDetectionService service.
// All implementations must embed UnimplementedUiDetectionServiceServer
// for forward compatibility
type UiDetectionServiceServer interface {
	// Runs the detection.
	Detect(context.Context, *UiDetectionRequest) (*UiDetectionResponse, error)
}

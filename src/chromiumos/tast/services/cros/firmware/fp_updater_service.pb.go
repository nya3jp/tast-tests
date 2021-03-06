// Code generated by protoc-gen-go. DO NOT EDIT.
// source: fp_updater_service.proto

package firmware

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	empty "github.com/golang/protobuf/ptypes/empty"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type ReadFpUpdaterLogsResponse struct {
	LatestLog            string   `protobuf:"bytes,1,opt,name=latest_log,json=latestLog,proto3" json:"latest_log,omitempty"`
	PreviousLog          string   `protobuf:"bytes,2,opt,name=previous_log,json=previousLog,proto3" json:"previous_log,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReadFpUpdaterLogsResponse) Reset()         { *m = ReadFpUpdaterLogsResponse{} }
func (m *ReadFpUpdaterLogsResponse) String() string { return proto.CompactTextString(m) }
func (*ReadFpUpdaterLogsResponse) ProtoMessage()    {}
func (*ReadFpUpdaterLogsResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_2e883b776c0b6c70, []int{0}
}

func (m *ReadFpUpdaterLogsResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReadFpUpdaterLogsResponse.Unmarshal(m, b)
}
func (m *ReadFpUpdaterLogsResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReadFpUpdaterLogsResponse.Marshal(b, m, deterministic)
}
func (m *ReadFpUpdaterLogsResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReadFpUpdaterLogsResponse.Merge(m, src)
}
func (m *ReadFpUpdaterLogsResponse) XXX_Size() int {
	return xxx_messageInfo_ReadFpUpdaterLogsResponse.Size(m)
}
func (m *ReadFpUpdaterLogsResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ReadFpUpdaterLogsResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ReadFpUpdaterLogsResponse proto.InternalMessageInfo

func (m *ReadFpUpdaterLogsResponse) GetLatestLog() string {
	if m != nil {
		return m.LatestLog
	}
	return ""
}

func (m *ReadFpUpdaterLogsResponse) GetPreviousLog() string {
	if m != nil {
		return m.PreviousLog
	}
	return ""
}

func init() {
	proto.RegisterType((*ReadFpUpdaterLogsResponse)(nil), "tast.cros.firmware.ReadFpUpdaterLogsResponse")
}

func init() { proto.RegisterFile("fp_updater_service.proto", fileDescriptor_2e883b776c0b6c70) }

var fileDescriptor_2e883b776c0b6c70 = []byte{
	// 234 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x90, 0x41, 0x4b, 0xc4, 0x30,
	0x10, 0x85, 0x5d, 0x0f, 0xc2, 0x46, 0x41, 0xc9, 0x41, 0xd6, 0x15, 0x41, 0xf7, 0x20, 0x7b, 0x71,
	0x02, 0xfa, 0x0f, 0x04, 0x3d, 0xf5, 0x54, 0xf1, 0xb2, 0x20, 0x25, 0xdb, 0x9d, 0xc6, 0x42, 0xbb,
	0x13, 0x66, 0xd2, 0x8a, 0xff, 0x5e, 0x92, 0x58, 0x11, 0xc4, 0xeb, 0xf0, 0xde, 0xf7, 0xbe, 0x44,
	0x2d, 0x1a, 0x5f, 0x0d, 0x7e, 0x67, 0x03, 0x72, 0x25, 0xc8, 0x63, 0x5b, 0x23, 0x78, 0xa6, 0x40,
	0x5a, 0x07, 0x2b, 0x01, 0x6a, 0x26, 0x81, 0xa6, 0xe5, 0xfe, 0xc3, 0x32, 0x2e, 0x2f, 0x1d, 0x91,
	0xeb, 0xd0, 0xa4, 0xc4, 0x76, 0x68, 0x0c, 0xf6, 0x3e, 0x7c, 0xe6, 0xc2, 0xea, 0x4d, 0x5d, 0x94,
	0x68, 0x77, 0xcf, 0xfe, 0x35, 0xf3, 0x0a, 0x72, 0x52, 0xa2, 0x78, 0xda, 0x0b, 0xea, 0x2b, 0xa5,
	0x3a, 0x1b, 0x50, 0x42, 0xd5, 0x91, 0x5b, 0xcc, 0xae, 0x67, 0xeb, 0x79, 0x39, 0xcf, 0x97, 0x82,
	0x9c, 0xbe, 0x51, 0x27, 0x9e, 0x71, 0x6c, 0x69, 0x90, 0x14, 0x38, 0x4c, 0x81, 0xe3, 0xe9, 0x56,
	0x90, 0xbb, 0xdf, 0xab, 0xb3, 0x1f, 0xf4, 0x4b, 0x36, 0xd5, 0x1b, 0x75, 0x1a, 0x27, 0x7f, 0x0d,
	0xea, 0x73, 0xc8, 0x8e, 0x30, 0x39, 0xc2, 0x53, 0x74, 0x5c, 0xde, 0xc1, 0xdf, 0xf7, 0xc0, 0xbf,
	0xbe, 0xab, 0x83, 0xc7, 0xf5, 0xe6, 0xb6, 0x7e, 0x67, 0xea, 0xdb, 0xa1, 0x27, 0x31, 0xb1, 0x6c,
	0xbe, 0x3f, 0x48, 0x4c, 0xa4, 0x98, 0x89, 0xb2, 0x3d, 0x4a, 0x53, 0x0f, 0x5f, 0x01, 0x00, 0x00,
	0xff, 0xff, 0xb6, 0xda, 0x07, 0x30, 0x4c, 0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// FpUpdaterServiceClient is the client API for FpUpdaterService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type FpUpdaterServiceClient interface {
	// ReadUpdaterLogs reads the latest and previous logs from the fingerprint firmware updater.
	ReadUpdaterLogs(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*ReadFpUpdaterLogsResponse, error)
}

type fpUpdaterServiceClient struct {
	cc *grpc.ClientConn
}

func NewFpUpdaterServiceClient(cc *grpc.ClientConn) FpUpdaterServiceClient {
	return &fpUpdaterServiceClient{cc}
}

func (c *fpUpdaterServiceClient) ReadUpdaterLogs(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*ReadFpUpdaterLogsResponse, error) {
	out := new(ReadFpUpdaterLogsResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.firmware.FpUpdaterService/ReadUpdaterLogs", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FpUpdaterServiceServer is the server API for FpUpdaterService service.
type FpUpdaterServiceServer interface {
	// ReadUpdaterLogs reads the latest and previous logs from the fingerprint firmware updater.
	ReadUpdaterLogs(context.Context, *empty.Empty) (*ReadFpUpdaterLogsResponse, error)
}

// UnimplementedFpUpdaterServiceServer can be embedded to have forward compatible implementations.
type UnimplementedFpUpdaterServiceServer struct {
}

func (*UnimplementedFpUpdaterServiceServer) ReadUpdaterLogs(ctx context.Context, req *empty.Empty) (*ReadFpUpdaterLogsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadUpdaterLogs not implemented")
}

func RegisterFpUpdaterServiceServer(s *grpc.Server, srv FpUpdaterServiceServer) {
	s.RegisterService(&_FpUpdaterService_serviceDesc, srv)
}

func _FpUpdaterService_ReadUpdaterLogs_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(FpUpdaterServiceServer).ReadUpdaterLogs(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.firmware.FpUpdaterService/ReadUpdaterLogs",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(FpUpdaterServiceServer).ReadUpdaterLogs(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _FpUpdaterService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.firmware.FpUpdaterService",
	HandlerType: (*FpUpdaterServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ReadUpdaterLogs",
			Handler:    _FpUpdaterService_ReadUpdaterLogs_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "fp_updater_service.proto",
}

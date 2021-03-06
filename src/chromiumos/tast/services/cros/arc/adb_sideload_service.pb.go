// Code generated by protoc-gen-go. DO NOT EDIT.
// source: adb_sideload_service.proto

package arc

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

type SigninRequest struct {
	Key                  string   `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *SigninRequest) Reset()         { *m = SigninRequest{} }
func (m *SigninRequest) String() string { return proto.CompactTextString(m) }
func (*SigninRequest) ProtoMessage()    {}
func (*SigninRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_0841636c70d7af15, []int{0}
}

func (m *SigninRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SigninRequest.Unmarshal(m, b)
}
func (m *SigninRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SigninRequest.Marshal(b, m, deterministic)
}
func (m *SigninRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SigninRequest.Merge(m, src)
}
func (m *SigninRequest) XXX_Size() int {
	return xxx_messageInfo_SigninRequest.Size(m)
}
func (m *SigninRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_SigninRequest.DiscardUnknown(m)
}

var xxx_messageInfo_SigninRequest proto.InternalMessageInfo

func (m *SigninRequest) GetKey() string {
	if m != nil {
		return m.Key
	}
	return ""
}

type AdbSideloadServiceRequest struct {
	Action               string   `protobuf:"bytes,1,opt,name=action,proto3" json:"action,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AdbSideloadServiceRequest) Reset()         { *m = AdbSideloadServiceRequest{} }
func (m *AdbSideloadServiceRequest) String() string { return proto.CompactTextString(m) }
func (*AdbSideloadServiceRequest) ProtoMessage()    {}
func (*AdbSideloadServiceRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_0841636c70d7af15, []int{1}
}

func (m *AdbSideloadServiceRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AdbSideloadServiceRequest.Unmarshal(m, b)
}
func (m *AdbSideloadServiceRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AdbSideloadServiceRequest.Marshal(b, m, deterministic)
}
func (m *AdbSideloadServiceRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AdbSideloadServiceRequest.Merge(m, src)
}
func (m *AdbSideloadServiceRequest) XXX_Size() int {
	return xxx_messageInfo_AdbSideloadServiceRequest.Size(m)
}
func (m *AdbSideloadServiceRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_AdbSideloadServiceRequest.DiscardUnknown(m)
}

var xxx_messageInfo_AdbSideloadServiceRequest proto.InternalMessageInfo

func (m *AdbSideloadServiceRequest) GetAction() string {
	if m != nil {
		return m.Action
	}
	return ""
}

func init() {
	proto.RegisterType((*SigninRequest)(nil), "tast.cros.arc.SigninRequest")
	proto.RegisterType((*AdbSideloadServiceRequest)(nil), "tast.cros.arc.AdbSideloadServiceRequest")
}

func init() { proto.RegisterFile("adb_sideload_service.proto", fileDescriptor_0841636c70d7af15) }

var fileDescriptor_0841636c70d7af15 = []byte{
	// 253 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x90, 0xcd, 0x4a, 0x03, 0x31,
	0x10, 0xc7, 0x5d, 0x84, 0x82, 0x81, 0x82, 0xe4, 0x50, 0xec, 0xaa, 0x60, 0xd7, 0x4b, 0x4f, 0x09,
	0xd8, 0x27, 0x50, 0xa9, 0x0f, 0xd0, 0xdc, 0xbc, 0x94, 0x24, 0x3b, 0x8d, 0x83, 0xbb, 0x99, 0x9a,
	0x64, 0x85, 0xbe, 0xa4, 0xcf, 0x24, 0xdb, 0x4d, 0xa1, 0x55, 0x7a, 0x4b, 0x98, 0xf9, 0x7f, 0xfc,
	0x86, 0x95, 0xba, 0x36, 0xeb, 0x88, 0x35, 0x34, 0xa4, 0xeb, 0x75, 0x84, 0xf0, 0x8d, 0x16, 0xc4,
	0x36, 0x50, 0x22, 0x3e, 0x4e, 0x3a, 0x26, 0x61, 0x03, 0x45, 0xa1, 0x83, 0x2d, 0x6f, 0x1d, 0x91,
	0x6b, 0x40, 0xee, 0x87, 0xa6, 0xdb, 0x48, 0x68, 0xb7, 0x69, 0x37, 0xec, 0x56, 0x33, 0x36, 0x56,
	0xe8, 0x3c, 0xfa, 0x15, 0x7c, 0x75, 0x10, 0x13, 0xbf, 0x66, 0x97, 0x9f, 0xb0, 0xbb, 0x29, 0x1e,
	0x8a, 0xf9, 0xd5, 0xaa, 0x7f, 0x56, 0x0b, 0x36, 0x7d, 0xae, 0x8d, 0xca, 0x59, 0x6a, 0x88, 0x3a,
	0xac, 0x4f, 0xd8, 0x48, 0xdb, 0x84, 0xe4, 0xb3, 0x22, 0xff, 0x9e, 0x7e, 0x0a, 0xc6, 0xff, 0xab,
	0xb8, 0x62, 0x53, 0x05, 0x29, 0x8b, 0x8f, 0xe6, 0x6f, 0x8d, 0x76, 0xfc, 0x4e, 0x9c, 0x14, 0x17,
	0x27, 0xc5, 0xca, 0x89, 0x18, 0x38, 0xc4, 0x81, 0x43, 0x2c, 0x7b, 0x8e, 0xea, 0x82, 0x5b, 0x76,
	0xff, 0x4a, 0x7e, 0x83, 0xa1, 0x5d, 0x7a, 0x6d, 0x1a, 0xf4, 0xee, 0xc8, 0x19, 0xbd, 0xe3, 0xf3,
	0x3f, 0xc6, 0x67, 0x71, 0xce, 0x87, 0xbc, 0x3c, 0xbe, 0xcf, 0xec, 0x47, 0xa0, 0x16, 0xbb, 0x96,
	0xa2, 0xec, 0xfd, 0x64, 0xbe, 0x7a, 0x94, 0xbd, 0xb1, 0xd4, 0xc1, 0x9a, 0xd1, 0x5e, 0xb6, 0xf8,
	0x0d, 0x00, 0x00, 0xff, 0xff, 0x40, 0x64, 0xf9, 0x49, 0x9e, 0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// AdbSideloadServiceClient is the client API for AdbSideloadService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type AdbSideloadServiceClient interface {
	// WaitUntilCPUCoolDown internally calls power.WaitUntilCPUCoolDown on DUT
	// and waits until CPU is cooled down.
	SetRequestAdbSideloadFlag(ctx context.Context, in *SigninRequest, opts ...grpc.CallOption) (*empty.Empty, error)
	ConfirmEnablingAdbSideloading(ctx context.Context, in *AdbSideloadServiceRequest, opts ...grpc.CallOption) (*empty.Empty, error)
}

type adbSideloadServiceClient struct {
	cc *grpc.ClientConn
}

func NewAdbSideloadServiceClient(cc *grpc.ClientConn) AdbSideloadServiceClient {
	return &adbSideloadServiceClient{cc}
}

func (c *adbSideloadServiceClient) SetRequestAdbSideloadFlag(ctx context.Context, in *SigninRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.arc.AdbSideloadService/SetRequestAdbSideloadFlag", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *adbSideloadServiceClient) ConfirmEnablingAdbSideloading(ctx context.Context, in *AdbSideloadServiceRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.arc.AdbSideloadService/ConfirmEnablingAdbSideloading", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AdbSideloadServiceServer is the server API for AdbSideloadService service.
type AdbSideloadServiceServer interface {
	// WaitUntilCPUCoolDown internally calls power.WaitUntilCPUCoolDown on DUT
	// and waits until CPU is cooled down.
	SetRequestAdbSideloadFlag(context.Context, *SigninRequest) (*empty.Empty, error)
	ConfirmEnablingAdbSideloading(context.Context, *AdbSideloadServiceRequest) (*empty.Empty, error)
}

// UnimplementedAdbSideloadServiceServer can be embedded to have forward compatible implementations.
type UnimplementedAdbSideloadServiceServer struct {
}

func (*UnimplementedAdbSideloadServiceServer) SetRequestAdbSideloadFlag(ctx context.Context, req *SigninRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetRequestAdbSideloadFlag not implemented")
}
func (*UnimplementedAdbSideloadServiceServer) ConfirmEnablingAdbSideloading(ctx context.Context, req *AdbSideloadServiceRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ConfirmEnablingAdbSideloading not implemented")
}

func RegisterAdbSideloadServiceServer(s *grpc.Server, srv AdbSideloadServiceServer) {
	s.RegisterService(&_AdbSideloadService_serviceDesc, srv)
}

func _AdbSideloadService_SetRequestAdbSideloadFlag_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SigninRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AdbSideloadServiceServer).SetRequestAdbSideloadFlag(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.arc.AdbSideloadService/SetRequestAdbSideloadFlag",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AdbSideloadServiceServer).SetRequestAdbSideloadFlag(ctx, req.(*SigninRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AdbSideloadService_ConfirmEnablingAdbSideloading_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AdbSideloadServiceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AdbSideloadServiceServer).ConfirmEnablingAdbSideloading(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.arc.AdbSideloadService/ConfirmEnablingAdbSideloading",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AdbSideloadServiceServer).ConfirmEnablingAdbSideloading(ctx, req.(*AdbSideloadServiceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _AdbSideloadService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.arc.AdbSideloadService",
	HandlerType: (*AdbSideloadServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SetRequestAdbSideloadFlag",
			Handler:    _AdbSideloadService_SetRequestAdbSideloadFlag_Handler,
		},
		{
			MethodName: "ConfirmEnablingAdbSideloading",
			Handler:    _AdbSideloadService_ConfirmEnablingAdbSideloading_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "adb_sideload_service.proto",
}

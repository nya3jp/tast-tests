// Code generated by protoc-gen-go. DO NOT EDIT.
// source: power_menu_service.proto

package wilco

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

type IsPowerMenuPresentResponse struct {
	IsMenuPresent        bool     `protobuf:"varint,1,opt,name=is_menu_present,json=isMenuPresent,proto3" json:"is_menu_present,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *IsPowerMenuPresentResponse) Reset()         { *m = IsPowerMenuPresentResponse{} }
func (m *IsPowerMenuPresentResponse) String() string { return proto.CompactTextString(m) }
func (*IsPowerMenuPresentResponse) ProtoMessage()    {}
func (*IsPowerMenuPresentResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_6bff766209769325, []int{0}
}

func (m *IsPowerMenuPresentResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_IsPowerMenuPresentResponse.Unmarshal(m, b)
}
func (m *IsPowerMenuPresentResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_IsPowerMenuPresentResponse.Marshal(b, m, deterministic)
}
func (m *IsPowerMenuPresentResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IsPowerMenuPresentResponse.Merge(m, src)
}
func (m *IsPowerMenuPresentResponse) XXX_Size() int {
	return xxx_messageInfo_IsPowerMenuPresentResponse.Size(m)
}
func (m *IsPowerMenuPresentResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_IsPowerMenuPresentResponse.DiscardUnknown(m)
}

var xxx_messageInfo_IsPowerMenuPresentResponse proto.InternalMessageInfo

func (m *IsPowerMenuPresentResponse) GetIsMenuPresent() bool {
	if m != nil {
		return m.IsMenuPresent
	}
	return false
}

func init() {
	proto.RegisterType((*IsPowerMenuPresentResponse)(nil), "tast.cros.wilco.IsPowerMenuPresentResponse")
}

func init() { proto.RegisterFile("power_menu_service.proto", fileDescriptor_6bff766209769325) }

var fileDescriptor_6bff766209769325 = []byte{
	// 241 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x91, 0xb1, 0x4a, 0x04, 0x31,
	0x10, 0x86, 0x6f, 0x1b, 0xd1, 0x88, 0x9c, 0xa4, 0x90, 0x63, 0x6d, 0x64, 0x45, 0x11, 0x84, 0x09,
	0x68, 0x2d, 0x82, 0xa7, 0x85, 0x85, 0x72, 0x9c, 0x9d, 0x16, 0x87, 0x17, 0xc6, 0x33, 0xb0, 0xc9,
	0x84, 0x4c, 0xe2, 0xe2, 0x53, 0xfb, 0x0a, 0x92, 0x8d, 0x8a, 0x28, 0xdb, 0x5c, 0x9b, 0x99, 0x7c,
	0xff, 0x97, 0x3f, 0x62, 0xe2, 0xa9, 0xc3, 0xb0, 0xb0, 0xe8, 0xd2, 0x82, 0x31, 0xbc, 0x19, 0x8d,
	0xe0, 0x03, 0x45, 0x92, 0xe3, 0xf8, 0xcc, 0x11, 0x74, 0x20, 0x86, 0xce, 0xb4, 0x9a, 0xea, 0xfd,
	0x15, 0xd1, 0xaa, 0x45, 0xd5, 0x8f, 0x97, 0xe9, 0x45, 0xa1, 0xf5, 0xf1, 0xbd, 0x6c, 0x37, 0xd7,
	0xa2, 0xbe, 0xe5, 0x59, 0x66, 0xdd, 0xa1, 0x4b, 0xb3, 0x80, 0x8c, 0x2e, 0xce, 0x91, 0x3d, 0x39,
	0x46, 0x79, 0x2c, 0xc6, 0x86, 0x4b, 0x88, 0x2f, 0xa3, 0x49, 0x75, 0x50, 0x9d, 0x6c, 0xce, 0x77,
	0x0c, 0xff, 0xda, 0x3f, 0xfb, 0xa8, 0xc4, 0xee, 0x0f, 0xe4, 0xa1, 0xe8, 0xc8, 0x0b, 0xb1, 0x75,
	0x8f, 0xdd, 0xf4, 0x35, 0x90, 0x45, 0xb9, 0x07, 0xc5, 0x02, 0xbe, 0x2d, 0xe0, 0x26, 0x5b, 0xd4,
	0x03, 0xe7, 0xcd, 0x48, 0x5e, 0x8a, 0xed, 0x69, 0x4b, 0x8c, 0x6b, 0x03, 0x9e, 0x84, 0xfc, 0xff,
	0xb4, 0x41, 0xce, 0x29, 0xfc, 0xe9, 0x0d, 0x86, 0x7b, 0x69, 0x46, 0x57, 0x47, 0x8f, 0x87, 0x3a,
	0x8b, 0x99, 0x64, 0x89, 0x55, 0xbe, 0xaa, 0xbe, 0xbe, 0x81, 0x55, 0x66, 0xa8, 0x9e, 0xb1, 0xdc,
	0xe8, 0x53, 0xce, 0x3f, 0x03, 0x00, 0x00, 0xff, 0xff, 0xd9, 0x60, 0x08, 0x15, 0xaf, 0x01, 0x00,
	0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// PowerMenuServiceClient is the client API for PowerMenuService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type PowerMenuServiceClient interface {
	// New logs into a Chrome session as a fake user. Close must be called later
	// to clean up the associated resources.
	NewChrome(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error)
	// Close releases the resources obtained by New.
	CloseChrome(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error)
	// IsPowerMenuPresent returns a bool indicating the presence of the power menu
	IsPowerMenuPresent(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*IsPowerMenuPresentResponse, error)
}

type powerMenuServiceClient struct {
	cc *grpc.ClientConn
}

func NewPowerMenuServiceClient(cc *grpc.ClientConn) PowerMenuServiceClient {
	return &powerMenuServiceClient{cc}
}

func (c *powerMenuServiceClient) NewChrome(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.wilco.PowerMenuService/NewChrome", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *powerMenuServiceClient) CloseChrome(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.wilco.PowerMenuService/CloseChrome", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *powerMenuServiceClient) IsPowerMenuPresent(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*IsPowerMenuPresentResponse, error) {
	out := new(IsPowerMenuPresentResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.wilco.PowerMenuService/IsPowerMenuPresent", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PowerMenuServiceServer is the server API for PowerMenuService service.
type PowerMenuServiceServer interface {
	// New logs into a Chrome session as a fake user. Close must be called later
	// to clean up the associated resources.
	NewChrome(context.Context, *empty.Empty) (*empty.Empty, error)
	// Close releases the resources obtained by New.
	CloseChrome(context.Context, *empty.Empty) (*empty.Empty, error)
	// IsPowerMenuPresent returns a bool indicating the presence of the power menu
	IsPowerMenuPresent(context.Context, *empty.Empty) (*IsPowerMenuPresentResponse, error)
}

// UnimplementedPowerMenuServiceServer can be embedded to have forward compatible implementations.
type UnimplementedPowerMenuServiceServer struct {
}

func (*UnimplementedPowerMenuServiceServer) NewChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method NewChrome not implemented")
}
func (*UnimplementedPowerMenuServiceServer) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CloseChrome not implemented")
}
func (*UnimplementedPowerMenuServiceServer) IsPowerMenuPresent(ctx context.Context, req *empty.Empty) (*IsPowerMenuPresentResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method IsPowerMenuPresent not implemented")
}

func RegisterPowerMenuServiceServer(s *grpc.Server, srv PowerMenuServiceServer) {
	s.RegisterService(&_PowerMenuService_serviceDesc, srv)
}

func _PowerMenuService_NewChrome_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PowerMenuServiceServer).NewChrome(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.wilco.PowerMenuService/NewChrome",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PowerMenuServiceServer).NewChrome(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _PowerMenuService_CloseChrome_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PowerMenuServiceServer).CloseChrome(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.wilco.PowerMenuService/CloseChrome",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PowerMenuServiceServer).CloseChrome(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _PowerMenuService_IsPowerMenuPresent_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PowerMenuServiceServer).IsPowerMenuPresent(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.wilco.PowerMenuService/IsPowerMenuPresent",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PowerMenuServiceServer).IsPowerMenuPresent(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _PowerMenuService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.wilco.PowerMenuService",
	HandlerType: (*PowerMenuServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "NewChrome",
			Handler:    _PowerMenuService_NewChrome_Handler,
		},
		{
			MethodName: "CloseChrome",
			Handler:    _PowerMenuService_CloseChrome_Handler,
		},
		{
			MethodName: "IsPowerMenuPresent",
			Handler:    _PowerMenuService_IsPowerMenuPresent_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "power_menu_service.proto",
}

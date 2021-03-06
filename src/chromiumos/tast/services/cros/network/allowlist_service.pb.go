// Code generated by protoc-gen-go. DO NOT EDIT.
// source: allowlist_service.proto

package network

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

type SetupFirewallRequest struct {
	// Must be a valid port number. Only http/s connection from this port are
	// allowed by the firewall.
	AllowedPort          uint32   `protobuf:"varint,1,opt,name=allowed_port,json=allowedPort,proto3" json:"allowed_port,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *SetupFirewallRequest) Reset()         { *m = SetupFirewallRequest{} }
func (m *SetupFirewallRequest) String() string { return proto.CompactTextString(m) }
func (*SetupFirewallRequest) ProtoMessage()    {}
func (*SetupFirewallRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5fc9a00f565c110a, []int{0}
}

func (m *SetupFirewallRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SetupFirewallRequest.Unmarshal(m, b)
}
func (m *SetupFirewallRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SetupFirewallRequest.Marshal(b, m, deterministic)
}
func (m *SetupFirewallRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SetupFirewallRequest.Merge(m, src)
}
func (m *SetupFirewallRequest) XXX_Size() int {
	return xxx_messageInfo_SetupFirewallRequest.Size(m)
}
func (m *SetupFirewallRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_SetupFirewallRequest.DiscardUnknown(m)
}

var xxx_messageInfo_SetupFirewallRequest proto.InternalMessageInfo

func (m *SetupFirewallRequest) GetAllowedPort() uint32 {
	if m != nil {
		return m.AllowedPort
	}
	return 0
}

type GaiaLoginRequest struct {
	Username string `protobuf:"bytes,1,opt,name=username,proto3" json:"username,omitempty"`
	Password string `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	// Host and port of an HTTP proxy, formatted as "<host>:<port>". The new
	// instance of Chrome will point to the proxy via command line args.
	ProxyHostAndPort     string   `protobuf:"bytes,3,opt,name=proxy_host_and_port,json=proxyHostAndPort,proto3" json:"proxy_host_and_port,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GaiaLoginRequest) Reset()         { *m = GaiaLoginRequest{} }
func (m *GaiaLoginRequest) String() string { return proto.CompactTextString(m) }
func (*GaiaLoginRequest) ProtoMessage()    {}
func (*GaiaLoginRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5fc9a00f565c110a, []int{1}
}

func (m *GaiaLoginRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GaiaLoginRequest.Unmarshal(m, b)
}
func (m *GaiaLoginRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GaiaLoginRequest.Marshal(b, m, deterministic)
}
func (m *GaiaLoginRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GaiaLoginRequest.Merge(m, src)
}
func (m *GaiaLoginRequest) XXX_Size() int {
	return xxx_messageInfo_GaiaLoginRequest.Size(m)
}
func (m *GaiaLoginRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GaiaLoginRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GaiaLoginRequest proto.InternalMessageInfo

func (m *GaiaLoginRequest) GetUsername() string {
	if m != nil {
		return m.Username
	}
	return ""
}

func (m *GaiaLoginRequest) GetPassword() string {
	if m != nil {
		return m.Password
	}
	return ""
}

func (m *GaiaLoginRequest) GetProxyHostAndPort() string {
	if m != nil {
		return m.ProxyHostAndPort
	}
	return ""
}

type CheckArcAppInstalledRequest struct {
	AppName              string   `protobuf:"bytes,1,opt,name=app_name,json=appName,proto3" json:"app_name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CheckArcAppInstalledRequest) Reset()         { *m = CheckArcAppInstalledRequest{} }
func (m *CheckArcAppInstalledRequest) String() string { return proto.CompactTextString(m) }
func (*CheckArcAppInstalledRequest) ProtoMessage()    {}
func (*CheckArcAppInstalledRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5fc9a00f565c110a, []int{2}
}

func (m *CheckArcAppInstalledRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CheckArcAppInstalledRequest.Unmarshal(m, b)
}
func (m *CheckArcAppInstalledRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CheckArcAppInstalledRequest.Marshal(b, m, deterministic)
}
func (m *CheckArcAppInstalledRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CheckArcAppInstalledRequest.Merge(m, src)
}
func (m *CheckArcAppInstalledRequest) XXX_Size() int {
	return xxx_messageInfo_CheckArcAppInstalledRequest.Size(m)
}
func (m *CheckArcAppInstalledRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CheckArcAppInstalledRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CheckArcAppInstalledRequest proto.InternalMessageInfo

func (m *CheckArcAppInstalledRequest) GetAppName() string {
	if m != nil {
		return m.AppName
	}
	return ""
}

type CheckExtensionInstalledRequest struct {
	ExtensionTitle       string   `protobuf:"bytes,1,opt,name=extension_title,json=extensionTitle,proto3" json:"extension_title,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CheckExtensionInstalledRequest) Reset()         { *m = CheckExtensionInstalledRequest{} }
func (m *CheckExtensionInstalledRequest) String() string { return proto.CompactTextString(m) }
func (*CheckExtensionInstalledRequest) ProtoMessage()    {}
func (*CheckExtensionInstalledRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5fc9a00f565c110a, []int{3}
}

func (m *CheckExtensionInstalledRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CheckExtensionInstalledRequest.Unmarshal(m, b)
}
func (m *CheckExtensionInstalledRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CheckExtensionInstalledRequest.Marshal(b, m, deterministic)
}
func (m *CheckExtensionInstalledRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CheckExtensionInstalledRequest.Merge(m, src)
}
func (m *CheckExtensionInstalledRequest) XXX_Size() int {
	return xxx_messageInfo_CheckExtensionInstalledRequest.Size(m)
}
func (m *CheckExtensionInstalledRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CheckExtensionInstalledRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CheckExtensionInstalledRequest proto.InternalMessageInfo

func (m *CheckExtensionInstalledRequest) GetExtensionTitle() string {
	if m != nil {
		return m.ExtensionTitle
	}
	return ""
}

func init() {
	proto.RegisterType((*SetupFirewallRequest)(nil), "tast.cros.network.SetupFirewallRequest")
	proto.RegisterType((*GaiaLoginRequest)(nil), "tast.cros.network.GaiaLoginRequest")
	proto.RegisterType((*CheckArcAppInstalledRequest)(nil), "tast.cros.network.CheckArcAppInstalledRequest")
	proto.RegisterType((*CheckExtensionInstalledRequest)(nil), "tast.cros.network.CheckExtensionInstalledRequest")
}

func init() { proto.RegisterFile("allowlist_service.proto", fileDescriptor_5fc9a00f565c110a) }

var fileDescriptor_5fc9a00f565c110a = []byte{
	// 392 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x53, 0x4d, 0x8f, 0xd3, 0x30,
	0x10, 0x65, 0x17, 0x09, 0x76, 0x0d, 0x0b, 0xc5, 0xac, 0xd8, 0xa5, 0x2b, 0x21, 0x08, 0x42, 0xe5,
	0x82, 0x23, 0xe0, 0x02, 0xc7, 0x80, 0x16, 0x28, 0x42, 0x08, 0xa5, 0x9c, 0xb8, 0x04, 0x37, 0x9d,
	0xb6, 0x56, 0x1d, 0x8f, 0xb1, 0x27, 0xa4, 0xfd, 0xef, 0x1c, 0x50, 0xbe, 0x2a, 0xd8, 0x26, 0x3d,
	0x7a, 0xde, 0xbc, 0xf7, 0x46, 0xef, 0xc9, 0xec, 0x4c, 0x6a, 0x8d, 0x85, 0x56, 0x9e, 0x12, 0x0f,
	0xee, 0xb7, 0x4a, 0x41, 0x58, 0x87, 0x84, 0xfc, 0x1e, 0x49, 0x4f, 0x22, 0x75, 0xe8, 0x85, 0x01,
	0x2a, 0xd0, 0xad, 0x86, 0x17, 0x0b, 0xc4, 0x85, 0x86, 0xb0, 0x5a, 0x98, 0xe6, 0xf3, 0x10, 0x32,
	0x4b, 0x9b, 0x7a, 0x3f, 0x78, 0xcb, 0x4e, 0x27, 0x40, 0xb9, 0xfd, 0xa0, 0x1c, 0x14, 0x52, 0xeb,
	0x18, 0x7e, 0xe5, 0xe0, 0x89, 0x3f, 0x61, 0xb7, 0x2b, 0x0b, 0x98, 0x25, 0x16, 0x1d, 0x9d, 0x1f,
	0x3c, 0x3e, 0x78, 0x7e, 0x12, 0xdf, 0x6a, 0x66, 0xdf, 0xd0, 0x51, 0xb0, 0x61, 0x83, 0x8f, 0x52,
	0xc9, 0x2f, 0xb8, 0x50, 0xa6, 0xa5, 0x0d, 0xd9, 0x51, 0xee, 0xc1, 0x19, 0x99, 0x41, 0x45, 0x39,
	0x8e, 0xb7, 0xef, 0x12, 0xb3, 0xd2, 0xfb, 0x02, 0xdd, 0xec, 0xfc, 0xb0, 0xc6, 0xda, 0x37, 0x7f,
	0xc1, 0xee, 0x5b, 0x87, 0xeb, 0x4d, 0xb2, 0x44, 0x4f, 0x89, 0x34, 0x8d, 0xeb, 0xf5, 0x6a, 0x6d,
	0x50, 0x41, 0x9f, 0xd0, 0x53, 0x64, 0x6a, 0xeb, 0x37, 0xec, 0xe2, 0xfd, 0x12, 0xd2, 0x55, 0xe4,
	0xd2, 0xc8, 0xda, 0xb1, 0xf1, 0x24, 0xb5, 0x86, 0x59, 0x7b, 0xc5, 0x43, 0x76, 0x24, 0xad, 0x4d,
	0xfe, 0xb9, 0xe2, 0xa6, 0xb4, 0xf6, 0xab, 0xcc, 0x20, 0x18, 0xb3, 0x47, 0x15, 0xf3, 0x72, 0x4d,
	0x60, 0xbc, 0x42, 0xb3, 0x43, 0x1e, 0xb1, 0xbb, 0xd0, 0x82, 0x09, 0x29, 0xd2, 0xad, 0xc6, 0x9d,
	0xed, 0xf8, 0x7b, 0x39, 0x7d, 0xf5, 0xe7, 0x90, 0x0d, 0xa2, 0xb6, 0x86, 0x49, 0xdd, 0x02, 0x8f,
	0xd9, 0xc9, 0x7f, 0x79, 0xf2, 0x91, 0xd8, 0x69, 0x44, 0x74, 0x25, 0x3e, 0x7c, 0x20, 0xea, 0x9e,
	0x44, 0xdb, 0x93, 0xb8, 0x2c, 0x7b, 0x0a, 0xae, 0xf1, 0xcf, 0xec, 0x78, 0x1b, 0x34, 0x7f, 0xda,
	0xa1, 0x77, 0xb5, 0x86, 0x3d, 0x5a, 0x3f, 0xd9, 0x69, 0x57, 0x72, 0x5c, 0x74, 0xc8, 0xee, 0x89,
	0x78, 0x8f, 0xc3, 0x9c, 0x9d, 0xf5, 0x24, 0xcc, 0x5f, 0xf6, 0x99, 0xf4, 0xb6, 0xd1, 0xef, 0xf3,
	0x6e, 0xf4, 0xe3, 0x59, 0xba, 0x74, 0x98, 0xa9, 0x3c, 0x43, 0x1f, 0x96, 0xc2, 0x61, 0xf3, 0x15,
	0x7c, 0x58, 0x3a, 0x84, 0x8d, 0xc3, 0xf4, 0x46, 0x45, 0x7d, 0xfd, 0x37, 0x00, 0x00, 0xff, 0xff,
	0xed, 0xa5, 0xa6, 0xbd, 0x34, 0x03, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// AllowlistServiceClient is the client API for AllowlistService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type AllowlistServiceClient interface {
	// SetupFirewall sets up a firewall using `iptables`, blocking https and https
	// connections through the default ports (80,443). Only http/s connections
	// coming from a specified port are allowed.
	SetupFirewall(ctx context.Context, in *SetupFirewallRequest, opts ...grpc.CallOption) (*empty.Empty, error)
	// GaiaLogin starts a new Chrome instance behind a proxy and performs
	// Chrome OS login using the specified credentials.
	GaiaLogin(ctx context.Context, in *GaiaLoginRequest, opts ...grpc.CallOption) (*empty.Empty, error)
	// CheckArcAppInstalled verifies that a specified ARC app is installed.
	CheckArcAppInstalled(ctx context.Context, in *CheckArcAppInstalledRequest, opts ...grpc.CallOption) (*empty.Empty, error)
	// CheckExtensionInstalled verifies that specified extension is installed.
	CheckExtensionInstalled(ctx context.Context, in *CheckExtensionInstalledRequest, opts ...grpc.CallOption) (*empty.Empty, error)
}

type allowlistServiceClient struct {
	cc *grpc.ClientConn
}

func NewAllowlistServiceClient(cc *grpc.ClientConn) AllowlistServiceClient {
	return &allowlistServiceClient{cc}
}

func (c *allowlistServiceClient) SetupFirewall(ctx context.Context, in *SetupFirewallRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.network.AllowlistService/SetupFirewall", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *allowlistServiceClient) GaiaLogin(ctx context.Context, in *GaiaLoginRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.network.AllowlistService/GaiaLogin", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *allowlistServiceClient) CheckArcAppInstalled(ctx context.Context, in *CheckArcAppInstalledRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.network.AllowlistService/CheckArcAppInstalled", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *allowlistServiceClient) CheckExtensionInstalled(ctx context.Context, in *CheckExtensionInstalledRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.network.AllowlistService/CheckExtensionInstalled", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AllowlistServiceServer is the server API for AllowlistService service.
type AllowlistServiceServer interface {
	// SetupFirewall sets up a firewall using `iptables`, blocking https and https
	// connections through the default ports (80,443). Only http/s connections
	// coming from a specified port are allowed.
	SetupFirewall(context.Context, *SetupFirewallRequest) (*empty.Empty, error)
	// GaiaLogin starts a new Chrome instance behind a proxy and performs
	// Chrome OS login using the specified credentials.
	GaiaLogin(context.Context, *GaiaLoginRequest) (*empty.Empty, error)
	// CheckArcAppInstalled verifies that a specified ARC app is installed.
	CheckArcAppInstalled(context.Context, *CheckArcAppInstalledRequest) (*empty.Empty, error)
	// CheckExtensionInstalled verifies that specified extension is installed.
	CheckExtensionInstalled(context.Context, *CheckExtensionInstalledRequest) (*empty.Empty, error)
}

// UnimplementedAllowlistServiceServer can be embedded to have forward compatible implementations.
type UnimplementedAllowlistServiceServer struct {
}

func (*UnimplementedAllowlistServiceServer) SetupFirewall(ctx context.Context, req *SetupFirewallRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetupFirewall not implemented")
}
func (*UnimplementedAllowlistServiceServer) GaiaLogin(ctx context.Context, req *GaiaLoginRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GaiaLogin not implemented")
}
func (*UnimplementedAllowlistServiceServer) CheckArcAppInstalled(ctx context.Context, req *CheckArcAppInstalledRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckArcAppInstalled not implemented")
}
func (*UnimplementedAllowlistServiceServer) CheckExtensionInstalled(ctx context.Context, req *CheckExtensionInstalledRequest) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckExtensionInstalled not implemented")
}

func RegisterAllowlistServiceServer(s *grpc.Server, srv AllowlistServiceServer) {
	s.RegisterService(&_AllowlistService_serviceDesc, srv)
}

func _AllowlistService_SetupFirewall_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetupFirewallRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AllowlistServiceServer).SetupFirewall(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.network.AllowlistService/SetupFirewall",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AllowlistServiceServer).SetupFirewall(ctx, req.(*SetupFirewallRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AllowlistService_GaiaLogin_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GaiaLoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AllowlistServiceServer).GaiaLogin(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.network.AllowlistService/GaiaLogin",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AllowlistServiceServer).GaiaLogin(ctx, req.(*GaiaLoginRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AllowlistService_CheckArcAppInstalled_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckArcAppInstalledRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AllowlistServiceServer).CheckArcAppInstalled(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.network.AllowlistService/CheckArcAppInstalled",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AllowlistServiceServer).CheckArcAppInstalled(ctx, req.(*CheckArcAppInstalledRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AllowlistService_CheckExtensionInstalled_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckExtensionInstalledRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AllowlistServiceServer).CheckExtensionInstalled(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.network.AllowlistService/CheckExtensionInstalled",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AllowlistServiceServer).CheckExtensionInstalled(ctx, req.(*CheckExtensionInstalledRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _AllowlistService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.network.AllowlistService",
	HandlerType: (*AllowlistServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SetupFirewall",
			Handler:    _AllowlistService_SetupFirewall_Handler,
		},
		{
			MethodName: "GaiaLogin",
			Handler:    _AllowlistService_GaiaLogin_Handler,
		},
		{
			MethodName: "CheckArcAppInstalled",
			Handler:    _AllowlistService_CheckArcAppInstalled_Handler,
		},
		{
			MethodName: "CheckExtensionInstalled",
			Handler:    _AllowlistService_CheckExtensionInstalled_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "allowlist_service.proto",
}

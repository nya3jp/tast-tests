// Code generated by protoc-gen-go. DO NOT EDIT.
// source: diag_service.proto

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

type RunRoutineRequest struct {
	// The name of the routine to run.
	Routine              string   `protobuf:"bytes,1,opt,name=routine,proto3" json:"routine,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RunRoutineRequest) Reset()         { *m = RunRoutineRequest{} }
func (m *RunRoutineRequest) String() string { return proto.CompactTextString(m) }
func (*RunRoutineRequest) ProtoMessage()    {}
func (*RunRoutineRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf4ac34ae6645245, []int{0}
}

func (m *RunRoutineRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RunRoutineRequest.Unmarshal(m, b)
}
func (m *RunRoutineRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RunRoutineRequest.Marshal(b, m, deterministic)
}
func (m *RunRoutineRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunRoutineRequest.Merge(m, src)
}
func (m *RunRoutineRequest) XXX_Size() int {
	return xxx_messageInfo_RunRoutineRequest.Size(m)
}
func (m *RunRoutineRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_RunRoutineRequest.DiscardUnknown(m)
}

var xxx_messageInfo_RunRoutineRequest proto.InternalMessageInfo

func (m *RunRoutineRequest) GetRoutine() string {
	if m != nil {
		return m.Routine
	}
	return ""
}

type RoutineResult struct {
	// The verdict of running the routine.
	Verdict int32 `protobuf:"varint,1,opt,name=verdict,proto3" json:"verdict,omitempty"`
	// List of routine problems if they exist.
	Problems             []uint32 `protobuf:"varint,2,rep,packed,name=problems,proto3" json:"problems,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RoutineResult) Reset()         { *m = RoutineResult{} }
func (m *RoutineResult) String() string { return proto.CompactTextString(m) }
func (*RoutineResult) ProtoMessage()    {}
func (*RoutineResult) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf4ac34ae6645245, []int{1}
}

func (m *RoutineResult) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RoutineResult.Unmarshal(m, b)
}
func (m *RoutineResult) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RoutineResult.Marshal(b, m, deterministic)
}
func (m *RoutineResult) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RoutineResult.Merge(m, src)
}
func (m *RoutineResult) XXX_Size() int {
	return xxx_messageInfo_RoutineResult.Size(m)
}
func (m *RoutineResult) XXX_DiscardUnknown() {
	xxx_messageInfo_RoutineResult.DiscardUnknown(m)
}

var xxx_messageInfo_RoutineResult proto.InternalMessageInfo

func (m *RoutineResult) GetVerdict() int32 {
	if m != nil {
		return m.Verdict
	}
	return 0
}

func (m *RoutineResult) GetProblems() []uint32 {
	if m != nil {
		return m.Problems
	}
	return nil
}

func init() {
	proto.RegisterType((*RunRoutineRequest)(nil), "tast.cros.network.RunRoutineRequest")
	proto.RegisterType((*RoutineResult)(nil), "tast.cros.network.RoutineResult")
}

func init() { proto.RegisterFile("diag_service.proto", fileDescriptor_cf4ac34ae6645245) }

var fileDescriptor_cf4ac34ae6645245 = []byte{
	// 274 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x91, 0x4d, 0x4b, 0xc3, 0x40,
	0x10, 0x86, 0x1b, 0xa5, 0x7e, 0x2c, 0x56, 0xe8, 0x1e, 0x24, 0xc4, 0x4b, 0x08, 0x8a, 0xbd, 0xb8,
	0x0b, 0x7a, 0xf2, 0xe6, 0x57, 0x0f, 0x5e, 0x44, 0xb6, 0xe0, 0xc1, 0x8b, 0x24, 0xe9, 0x18, 0x17,
	0x93, 0x4c, 0xdc, 0x9d, 0xad, 0xf8, 0x83, 0xfd, 0x1f, 0x92, 0xa4, 0xa9, 0x42, 0xf4, 0xd2, 0xe3,
	0xbc, 0x3c, 0x2f, 0x3c, 0xbc, 0xc3, 0xf8, 0x5c, 0xc7, 0xd9, 0xb3, 0x05, 0xb3, 0xd0, 0x29, 0x88,
	0xca, 0x20, 0x21, 0x1f, 0x53, 0x6c, 0x49, 0xa4, 0x06, 0xad, 0x28, 0x81, 0x3e, 0xd0, 0xbc, 0x05,
	0x87, 0x19, 0x62, 0x96, 0x83, 0x6c, 0x80, 0xc4, 0xbd, 0x48, 0x28, 0x2a, 0xfa, 0x6c, 0xf9, 0xe8,
	0x94, 0x8d, 0x95, 0x2b, 0x15, 0x3a, 0xd2, 0x25, 0x28, 0x78, 0x77, 0x60, 0x89, 0xfb, 0x6c, 0xdb,
	0xb4, 0x89, 0xef, 0x85, 0xde, 0x64, 0x57, 0x75, 0x67, 0x34, 0x65, 0xa3, 0x15, 0x6b, 0x5d, 0xde,
	0xa0, 0x0b, 0x30, 0x73, 0x9d, 0x52, 0x83, 0x0e, 0x55, 0x77, 0xf2, 0x80, 0xed, 0x54, 0x06, 0x93,
	0x1c, 0x0a, 0xeb, 0x6f, 0x84, 0x9b, 0x93, 0x91, 0x5a, 0xdd, 0x67, 0x5f, 0x1e, 0xdb, 0xbf, 0x07,
	0xba, 0xd5, 0x71, 0x36, 0x6b, 0xf5, 0xf9, 0x25, 0xdb, 0x9b, 0x01, 0xb9, 0xaa, 0xce, 0xae, 0x1e,
	0xee, 0xf8, 0x81, 0x68, 0xb5, 0x45, 0xa7, 0x2d, 0xa6, 0xb5, 0x76, 0xf0, 0x4f, 0x1e, 0x0d, 0xf8,
	0x05, 0x1b, 0xde, 0xe4, 0x68, 0x61, 0x8d, 0xea, 0x23, 0x63, 0x3f, 0x2b, 0xf0, 0x23, 0xd1, 0x1b,
	0x51, 0xf4, 0x46, 0x0a, 0xc2, 0xbf, 0xa8, 0xdf, 0xdb, 0x44, 0x83, 0xeb, 0x93, 0xa7, 0xe3, 0xf4,
	0xd5, 0x60, 0xa1, 0x5d, 0x81, 0x56, 0xd6, 0xbc, 0x5c, 0xbe, 0xcb, 0xca, 0xba, 0x28, 0x97, 0xc5,
	0x64, 0xab, 0x51, 0x3a, 0xff, 0x0e, 0x00, 0x00, 0xff, 0xff, 0xb1, 0xd7, 0xf4, 0x40, 0xd3, 0x01,
	0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// NetDiagServiceClient is the client API for NetDiagService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type NetDiagServiceClient interface {
	// SetupDiagAPI creates a new chrome instance and launches the connectivity
	// diagnostics application to be used for running the network diagnostics.
	SetupDiagAPI(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error)
	// Close will close the connectivity diagnostics application and the
	// underlying Chrome instance.
	Close(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error)
	// RunRoutine will run the specified network diagnostic routine and return the
	// result.
	RunRoutine(ctx context.Context, in *RunRoutineRequest, opts ...grpc.CallOption) (*RoutineResult, error)
}

type netDiagServiceClient struct {
	cc *grpc.ClientConn
}

func NewNetDiagServiceClient(cc *grpc.ClientConn) NetDiagServiceClient {
	return &netDiagServiceClient{cc}
}

func (c *netDiagServiceClient) SetupDiagAPI(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.network.NetDiagService/SetupDiagAPI", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netDiagServiceClient) Close(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.network.NetDiagService/Close", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netDiagServiceClient) RunRoutine(ctx context.Context, in *RunRoutineRequest, opts ...grpc.CallOption) (*RoutineResult, error) {
	out := new(RoutineResult)
	err := c.cc.Invoke(ctx, "/tast.cros.network.NetDiagService/RunRoutine", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NetDiagServiceServer is the server API for NetDiagService service.
type NetDiagServiceServer interface {
	// SetupDiagAPI creates a new chrome instance and launches the connectivity
	// diagnostics application to be used for running the network diagnostics.
	SetupDiagAPI(context.Context, *empty.Empty) (*empty.Empty, error)
	// Close will close the connectivity diagnostics application and the
	// underlying Chrome instance.
	Close(context.Context, *empty.Empty) (*empty.Empty, error)
	// RunRoutine will run the specified network diagnostic routine and return the
	// result.
	RunRoutine(context.Context, *RunRoutineRequest) (*RoutineResult, error)
}

// UnimplementedNetDiagServiceServer can be embedded to have forward compatible implementations.
type UnimplementedNetDiagServiceServer struct {
}

func (*UnimplementedNetDiagServiceServer) SetupDiagAPI(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetupDiagAPI not implemented")
}
func (*UnimplementedNetDiagServiceServer) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Close not implemented")
}
func (*UnimplementedNetDiagServiceServer) RunRoutine(ctx context.Context, req *RunRoutineRequest) (*RoutineResult, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunRoutine not implemented")
}

func RegisterNetDiagServiceServer(s *grpc.Server, srv NetDiagServiceServer) {
	s.RegisterService(&_NetDiagService_serviceDesc, srv)
}

func _NetDiagService_SetupDiagAPI_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetDiagServiceServer).SetupDiagAPI(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.network.NetDiagService/SetupDiagAPI",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetDiagServiceServer).SetupDiagAPI(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _NetDiagService_Close_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetDiagServiceServer).Close(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.network.NetDiagService/Close",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetDiagServiceServer).Close(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _NetDiagService_RunRoutine_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunRoutineRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetDiagServiceServer).RunRoutine(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.network.NetDiagService/RunRoutine",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetDiagServiceServer).RunRoutine(ctx, req.(*RunRoutineRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _NetDiagService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.network.NetDiagService",
	HandlerType: (*NetDiagServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SetupDiagAPI",
			Handler:    _NetDiagService_SetupDiagAPI_Handler,
		},
		{
			MethodName: "Close",
			Handler:    _NetDiagService_Close_Handler,
		},
		{
			MethodName: "RunRoutine",
			Handler:    _NetDiagService_RunRoutine_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "diag_service.proto",
}

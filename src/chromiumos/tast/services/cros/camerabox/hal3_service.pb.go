// Code generated by protoc-gen-go. DO NOT EDIT.
// source: hal3_service.proto

package camerabox

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
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

type HAL3CameraTest int32

const (
	HAL3CameraTest_DEVICE        HAL3CameraTest = 0
	HAL3CameraTest_FRAME         HAL3CameraTest = 1
	HAL3CameraTest_JDA           HAL3CameraTest = 2
	HAL3CameraTest_JEA           HAL3CameraTest = 3
	HAL3CameraTest_MODULE        HAL3CameraTest = 4
	HAL3CameraTest_PERF          HAL3CameraTest = 5
	HAL3CameraTest_PREVIEW       HAL3CameraTest = 6
	HAL3CameraTest_RECORDING     HAL3CameraTest = 7
	HAL3CameraTest_STILL_CAPTURE HAL3CameraTest = 8
	HAL3CameraTest_STREAM        HAL3CameraTest = 9
)

var HAL3CameraTest_name = map[int32]string{
	0: "DEVICE",
	1: "FRAME",
	2: "JDA",
	3: "JEA",
	4: "MODULE",
	5: "PERF",
	6: "PREVIEW",
	7: "RECORDING",
	8: "STILL_CAPTURE",
	9: "STREAM",
}

var HAL3CameraTest_value = map[string]int32{
	"DEVICE":        0,
	"FRAME":         1,
	"JDA":           2,
	"JEA":           3,
	"MODULE":        4,
	"PERF":          5,
	"PREVIEW":       6,
	"RECORDING":     7,
	"STILL_CAPTURE": 8,
	"STREAM":        9,
}

func (x HAL3CameraTest) String() string {
	return proto.EnumName(HAL3CameraTest_name, int32(x))
}

func (HAL3CameraTest) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_622c68ca9bf6b99e, []int{0}
}

type TestResult int32

const (
	TestResult_TEST_RESULT_UNSET TestResult = 0
	// Test is passed.
	TestResult_TEST_RESULT_PASSED TestResult = 1
	// Test is failed.
	TestResult_TEST_RESULT_FAILED TestResult = 2
)

var TestResult_name = map[int32]string{
	0: "TEST_RESULT_UNSET",
	1: "TEST_RESULT_PASSED",
	2: "TEST_RESULT_FAILED",
}

var TestResult_value = map[string]int32{
	"TEST_RESULT_UNSET":  0,
	"TEST_RESULT_PASSED": 1,
	"TEST_RESULT_FAILED": 2,
}

func (x TestResult) String() string {
	return proto.EnumName(TestResult_name, int32(x))
}

func (TestResult) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_622c68ca9bf6b99e, []int{1}
}

type RunTestRequest struct {
	// Type of test to be run.
	Test HAL3CameraTest `protobuf:"varint,1,opt,name=test,proto3,enum=tast.cros.camerabox.HAL3CameraTest" json:"test,omitempty"`
	// Facing of camera to be tested.
	Facing               Facing   `protobuf:"varint,2,opt,name=facing,proto3,enum=tast.cros.camerabox.Facing" json:"facing,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RunTestRequest) Reset()         { *m = RunTestRequest{} }
func (m *RunTestRequest) String() string { return proto.CompactTextString(m) }
func (*RunTestRequest) ProtoMessage()    {}
func (*RunTestRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_622c68ca9bf6b99e, []int{0}
}

func (m *RunTestRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RunTestRequest.Unmarshal(m, b)
}
func (m *RunTestRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RunTestRequest.Marshal(b, m, deterministic)
}
func (m *RunTestRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunTestRequest.Merge(m, src)
}
func (m *RunTestRequest) XXX_Size() int {
	return xxx_messageInfo_RunTestRequest.Size(m)
}
func (m *RunTestRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_RunTestRequest.DiscardUnknown(m)
}

var xxx_messageInfo_RunTestRequest proto.InternalMessageInfo

func (m *RunTestRequest) GetTest() HAL3CameraTest {
	if m != nil {
		return m.Test
	}
	return HAL3CameraTest_DEVICE
}

func (m *RunTestRequest) GetFacing() Facing {
	if m != nil {
		return m.Facing
	}
	return Facing_FACING_UNSET
}

type RunTestResponse struct {
	Result TestResult `protobuf:"varint,1,opt,name=result,proto3,enum=tast.cros.camerabox.TestResult" json:"result,omitempty"`
	// Error message from running test.
	Error                string   `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RunTestResponse) Reset()         { *m = RunTestResponse{} }
func (m *RunTestResponse) String() string { return proto.CompactTextString(m) }
func (*RunTestResponse) ProtoMessage()    {}
func (*RunTestResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_622c68ca9bf6b99e, []int{1}
}

func (m *RunTestResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RunTestResponse.Unmarshal(m, b)
}
func (m *RunTestResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RunTestResponse.Marshal(b, m, deterministic)
}
func (m *RunTestResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RunTestResponse.Merge(m, src)
}
func (m *RunTestResponse) XXX_Size() int {
	return xxx_messageInfo_RunTestResponse.Size(m)
}
func (m *RunTestResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_RunTestResponse.DiscardUnknown(m)
}

var xxx_messageInfo_RunTestResponse proto.InternalMessageInfo

func (m *RunTestResponse) GetResult() TestResult {
	if m != nil {
		return m.Result
	}
	return TestResult_TEST_RESULT_UNSET
}

func (m *RunTestResponse) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

func init() {
	proto.RegisterEnum("tast.cros.camerabox.HAL3CameraTest", HAL3CameraTest_name, HAL3CameraTest_value)
	proto.RegisterEnum("tast.cros.camerabox.TestResult", TestResult_name, TestResult_value)
	proto.RegisterType((*RunTestRequest)(nil), "tast.cros.camerabox.RunTestRequest")
	proto.RegisterType((*RunTestResponse)(nil), "tast.cros.camerabox.RunTestResponse")
}

func init() { proto.RegisterFile("hal3_service.proto", fileDescriptor_622c68ca9bf6b99e) }

var fileDescriptor_622c68ca9bf6b99e = []byte{
	// 411 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x92, 0x4f, 0x6f, 0xd3, 0x40,
	0x10, 0xc5, 0xe3, 0x34, 0x71, 0x9a, 0x29, 0x0d, 0xdb, 0xe1, 0x8f, 0xaa, 0x72, 0x00, 0x05, 0x24,
	0xa0, 0x07, 0x47, 0x6a, 0x0e, 0x3d, 0x9b, 0x78, 0x02, 0x46, 0x4e, 0x1b, 0xed, 0x3a, 0x41, 0xe2,
	0x12, 0x5c, 0x6b, 0xa1, 0x91, 0xe2, 0x6c, 0xd9, 0x5d, 0x23, 0x4e, 0x7c, 0x00, 0x3e, 0x35, 0xf2,
	0xc6, 0x14, 0x8a, 0x2c, 0x6e, 0x9e, 0xd1, 0xfb, 0xcd, 0x3c, 0xef, 0x1b, 0xc0, 0xeb, 0x6c, 0x33,
	0x5e, 0x19, 0xa9, 0xbf, 0xad, 0x73, 0x19, 0xdc, 0x68, 0x65, 0x15, 0x3e, 0xb0, 0x99, 0xb1, 0x41,
	0xae, 0x95, 0x09, 0xf2, 0xac, 0x90, 0x3a, 0xbb, 0x52, 0xdf, 0x4f, 0xee, 0xe5, 0xaa, 0x28, 0xd4,
	0x76, 0x27, 0x19, 0xfe, 0x80, 0x01, 0x2f, 0xb7, 0xa9, 0x34, 0x96, 0xcb, 0xaf, 0xa5, 0x34, 0x16,
	0xcf, 0xa1, 0x63, 0xa5, 0xb1, 0xc7, 0xde, 0x33, 0xef, 0xd5, 0xe0, 0xec, 0x79, 0xd0, 0x30, 0x23,
	0x78, 0x17, 0x26, 0xe3, 0x89, 0xab, 0x1c, 0xe9, 0x00, 0x1c, 0x83, 0xff, 0x39, 0xcb, 0xd7, 0xdb,
	0x2f, 0xc7, 0x6d, 0x87, 0x3e, 0x69, 0x44, 0xa7, 0x4e, 0xc2, 0x6b, 0xe9, 0xf0, 0x13, 0xdc, 0xbf,
	0xdd, 0x6f, 0x6e, 0xd4, 0xd6, 0x48, 0x3c, 0x07, 0x5f, 0x4b, 0x53, 0x6e, 0x7e, 0x5b, 0x78, 0xda,
	0x38, 0xa7, 0x46, 0xca, 0x8d, 0xe5, 0xb5, 0x1c, 0x1f, 0x42, 0x57, 0x6a, 0xad, 0xb4, 0xdb, 0xdf,
	0xe7, 0xbb, 0xe2, 0xf4, 0xa7, 0x07, 0x83, 0xbb, 0x7e, 0x11, 0xc0, 0x8f, 0x68, 0x19, 0x4f, 0x88,
	0xb5, 0xb0, 0x0f, 0xdd, 0x29, 0x0f, 0x67, 0xc4, 0x3c, 0xec, 0xc1, 0xde, 0xfb, 0x28, 0x64, 0x6d,
	0xf7, 0x41, 0x21, 0xdb, 0xab, 0x84, 0xb3, 0xcb, 0x68, 0x91, 0x10, 0xeb, 0xe0, 0x3e, 0x74, 0xe6,
	0xc4, 0xa7, 0xac, 0x8b, 0x07, 0xd0, 0x9b, 0x73, 0x5a, 0xc6, 0xf4, 0x81, 0xf9, 0x78, 0x08, 0x7d,
	0x4e, 0x93, 0x4b, 0x1e, 0xc5, 0x17, 0x6f, 0x59, 0x0f, 0x8f, 0xe0, 0x50, 0xa4, 0x71, 0x92, 0xac,
	0x26, 0xe1, 0x3c, 0x5d, 0x70, 0x62, 0xfb, 0xd5, 0x10, 0x91, 0x72, 0x0a, 0x67, 0xac, 0x7f, 0x2a,
	0x00, 0xfe, 0x18, 0xc7, 0x47, 0x70, 0x94, 0x92, 0x48, 0x57, 0x9c, 0xc4, 0x22, 0x49, 0x57, 0x8b,
	0x0b, 0x41, 0x29, 0x6b, 0xe1, 0x63, 0xc0, 0xbf, 0xdb, 0xf3, 0x50, 0x08, 0x8a, 0x98, 0xf7, 0x6f,
	0x7f, 0x1a, 0xc6, 0x09, 0x45, 0xac, 0x7d, 0x26, 0xe1, 0xa0, 0xfa, 0x41, 0xb1, 0xcb, 0x1e, 0x97,
	0xd0, 0xab, 0x9f, 0x14, 0x9b, 0xd3, 0xbb, 0x1b, 0xf8, 0xc9, 0x8b, 0xff, 0x8b, 0x76, 0xa9, 0x0c,
	0x5b, 0x6f, 0x5e, 0x7f, 0x7c, 0x99, 0x5f, 0x6b, 0x55, 0xac, 0xcb, 0x42, 0x99, 0x51, 0xc5, 0x8c,
	0xea, 0x73, 0x33, 0xa3, 0x0a, 0x1e, 0xdd, 0xc2, 0x57, 0xbe, 0x3b, 0xae, 0xf1, 0xaf, 0x00, 0x00,
	0x00, 0xff, 0xff, 0xf5, 0x18, 0xbe, 0x51, 0x95, 0x02, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// HAL3ServiceClient is the client API for HAL3Service service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type HAL3ServiceClient interface {
	// RunTest runs specific HAL3 camera test on target facing camera.
	RunTest(ctx context.Context, in *RunTestRequest, opts ...grpc.CallOption) (*RunTestResponse, error)
}

type hAL3ServiceClient struct {
	cc *grpc.ClientConn
}

func NewHAL3ServiceClient(cc *grpc.ClientConn) HAL3ServiceClient {
	return &hAL3ServiceClient{cc}
}

func (c *hAL3ServiceClient) RunTest(ctx context.Context, in *RunTestRequest, opts ...grpc.CallOption) (*RunTestResponse, error) {
	out := new(RunTestResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.camerabox.HAL3Service/RunTest", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// HAL3ServiceServer is the server API for HAL3Service service.
type HAL3ServiceServer interface {
	// RunTest runs specific HAL3 camera test on target facing camera.
	RunTest(context.Context, *RunTestRequest) (*RunTestResponse, error)
}

// UnimplementedHAL3ServiceServer can be embedded to have forward compatible implementations.
type UnimplementedHAL3ServiceServer struct {
}

func (*UnimplementedHAL3ServiceServer) RunTest(ctx context.Context, req *RunTestRequest) (*RunTestResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunTest not implemented")
}

func RegisterHAL3ServiceServer(s *grpc.Server, srv HAL3ServiceServer) {
	s.RegisterService(&_HAL3Service_serviceDesc, srv)
}

func _HAL3Service_RunTest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunTestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HAL3ServiceServer).RunTest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.camerabox.HAL3Service/RunTest",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HAL3ServiceServer).RunTest(ctx, req.(*RunTestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _HAL3Service_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.camerabox.HAL3Service",
	HandlerType: (*HAL3ServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RunTest",
			Handler:    _HAL3Service_RunTest_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "hal3_service.proto",
}

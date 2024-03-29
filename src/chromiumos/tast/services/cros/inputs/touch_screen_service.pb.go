// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.11.4
// source: touch_screen_service.proto

package inputs

import (
	context "context"
	empty "github.com/golang/protobuf/ptypes/empty"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// FindPhysicalTouchscreenResponse provides the path to /dev/input/event* for a physical trackscreen.
type FindPhysicalTouchscreenResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
}

func (x *FindPhysicalTouchscreenResponse) Reset() {
	*x = FindPhysicalTouchscreenResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_touch_screen_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FindPhysicalTouchscreenResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FindPhysicalTouchscreenResponse) ProtoMessage() {}

func (x *FindPhysicalTouchscreenResponse) ProtoReflect() protoreflect.Message {
	mi := &file_touch_screen_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FindPhysicalTouchscreenResponse.ProtoReflect.Descriptor instead.
func (*FindPhysicalTouchscreenResponse) Descriptor() ([]byte, []int) {
	return file_touch_screen_service_proto_rawDescGZIP(), []int{0}
}

func (x *FindPhysicalTouchscreenResponse) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

var File_touch_screen_service_proto protoreflect.FileDescriptor

var file_touch_screen_service_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x74, 0x6f, 0x75, 0x63, 0x68, 0x5f, 0x73, 0x63, 0x72, 0x65, 0x65, 0x6e, 0x5f, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x74, 0x61,
	0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x69, 0x6e, 0x70, 0x75, 0x74, 0x73, 0x1a, 0x1b,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x35, 0x0a, 0x1f, 0x46,
	0x69, 0x6e, 0x64, 0x50, 0x68, 0x79, 0x73, 0x69, 0x63, 0x61, 0x6c, 0x54, 0x6f, 0x75, 0x63, 0x68,
	0x73, 0x63, 0x72, 0x65, 0x65, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x12,
	0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61,
	0x74, 0x68, 0x32, 0xc0, 0x02, 0x0a, 0x12, 0x54, 0x6f, 0x75, 0x63, 0x68, 0x73, 0x63, 0x72, 0x65,
	0x65, 0x6e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x3d, 0x0a, 0x09, 0x4e, 0x65, 0x77,
	0x43, 0x68, 0x72, 0x6f, 0x6d, 0x65, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x16,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x3f, 0x0a, 0x0b, 0x43, 0x6c, 0x6f, 0x73,
	0x65, 0x43, 0x68, 0x72, 0x6f, 0x6d, 0x65, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a,
	0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x66, 0x0a, 0x17, 0x46, 0x69, 0x6e,
	0x64, 0x50, 0x68, 0x79, 0x73, 0x69, 0x63, 0x61, 0x6c, 0x54, 0x6f, 0x75, 0x63, 0x68, 0x73, 0x63,
	0x72, 0x65, 0x65, 0x6e, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x31, 0x2e, 0x74,
	0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x69, 0x6e, 0x70, 0x75, 0x74, 0x73, 0x2e,
	0x46, 0x69, 0x6e, 0x64, 0x50, 0x68, 0x79, 0x73, 0x69, 0x63, 0x61, 0x6c, 0x54, 0x6f, 0x75, 0x63,
	0x68, 0x73, 0x63, 0x72, 0x65, 0x65, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x42, 0x0a, 0x0e, 0x54, 0x6f, 0x75, 0x63, 0x68, 0x73, 0x63, 0x72, 0x65, 0x65, 0x6e,
	0x54, 0x61, 0x70, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x16, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d,
	0x70, 0x74, 0x79, 0x22, 0x00, 0x42, 0x26, 0x5a, 0x24, 0x63, 0x68, 0x72, 0x6f, 0x6d, 0x69, 0x75,
	0x6d, 0x6f, 0x73, 0x2f, 0x74, 0x61, 0x73, 0x74, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x73, 0x2f, 0x63, 0x72, 0x6f, 0x73, 0x2f, 0x69, 0x6e, 0x70, 0x75, 0x74, 0x73, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_touch_screen_service_proto_rawDescOnce sync.Once
	file_touch_screen_service_proto_rawDescData = file_touch_screen_service_proto_rawDesc
)

func file_touch_screen_service_proto_rawDescGZIP() []byte {
	file_touch_screen_service_proto_rawDescOnce.Do(func() {
		file_touch_screen_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_touch_screen_service_proto_rawDescData)
	})
	return file_touch_screen_service_proto_rawDescData
}

var file_touch_screen_service_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_touch_screen_service_proto_goTypes = []interface{}{
	(*FindPhysicalTouchscreenResponse)(nil), // 0: tast.cros.inputs.FindPhysicalTouchscreenResponse
	(*empty.Empty)(nil),                     // 1: google.protobuf.Empty
}
var file_touch_screen_service_proto_depIdxs = []int32{
	1, // 0: tast.cros.inputs.TouchscreenService.NewChrome:input_type -> google.protobuf.Empty
	1, // 1: tast.cros.inputs.TouchscreenService.CloseChrome:input_type -> google.protobuf.Empty
	1, // 2: tast.cros.inputs.TouchscreenService.FindPhysicalTouchscreen:input_type -> google.protobuf.Empty
	1, // 3: tast.cros.inputs.TouchscreenService.TouchscreenTap:input_type -> google.protobuf.Empty
	1, // 4: tast.cros.inputs.TouchscreenService.NewChrome:output_type -> google.protobuf.Empty
	1, // 5: tast.cros.inputs.TouchscreenService.CloseChrome:output_type -> google.protobuf.Empty
	0, // 6: tast.cros.inputs.TouchscreenService.FindPhysicalTouchscreen:output_type -> tast.cros.inputs.FindPhysicalTouchscreenResponse
	1, // 7: tast.cros.inputs.TouchscreenService.TouchscreenTap:output_type -> google.protobuf.Empty
	4, // [4:8] is the sub-list for method output_type
	0, // [0:4] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_touch_screen_service_proto_init() }
func file_touch_screen_service_proto_init() {
	if File_touch_screen_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_touch_screen_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FindPhysicalTouchscreenResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_touch_screen_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_touch_screen_service_proto_goTypes,
		DependencyIndexes: file_touch_screen_service_proto_depIdxs,
		MessageInfos:      file_touch_screen_service_proto_msgTypes,
	}.Build()
	File_touch_screen_service_proto = out.File
	file_touch_screen_service_proto_rawDesc = nil
	file_touch_screen_service_proto_goTypes = nil
	file_touch_screen_service_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// TouchscreenServiceClient is the client API for TouchscreenService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type TouchscreenServiceClient interface {
	// NewChrome logs into a Chrome session as a fake user. CloseChrome must be called later
	// to clean up the associated resources.
	NewChrome(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error)
	// CloseChrome releases the resources obtained by NewChrome.
	CloseChrome(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error)
	// FindPhysicalTouchscreen finds /dev/input/event* file for a physical touchscreen.
	FindPhysicalTouchscreen(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*FindPhysicalTouchscreenResponse, error)
	// TouchscreenTap injects a tap event to the touch screen.
	TouchscreenTap(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error)
}

type touchscreenServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTouchscreenServiceClient(cc grpc.ClientConnInterface) TouchscreenServiceClient {
	return &touchscreenServiceClient{cc}
}

func (c *touchscreenServiceClient) NewChrome(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.inputs.TouchscreenService/NewChrome", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *touchscreenServiceClient) CloseChrome(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.inputs.TouchscreenService/CloseChrome", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *touchscreenServiceClient) FindPhysicalTouchscreen(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*FindPhysicalTouchscreenResponse, error) {
	out := new(FindPhysicalTouchscreenResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.inputs.TouchscreenService/FindPhysicalTouchscreen", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *touchscreenServiceClient) TouchscreenTap(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.inputs.TouchscreenService/TouchscreenTap", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TouchscreenServiceServer is the server API for TouchscreenService service.
type TouchscreenServiceServer interface {
	// NewChrome logs into a Chrome session as a fake user. CloseChrome must be called later
	// to clean up the associated resources.
	NewChrome(context.Context, *empty.Empty) (*empty.Empty, error)
	// CloseChrome releases the resources obtained by NewChrome.
	CloseChrome(context.Context, *empty.Empty) (*empty.Empty, error)
	// FindPhysicalTouchscreen finds /dev/input/event* file for a physical touchscreen.
	FindPhysicalTouchscreen(context.Context, *empty.Empty) (*FindPhysicalTouchscreenResponse, error)
	// TouchscreenTap injects a tap event to the touch screen.
	TouchscreenTap(context.Context, *empty.Empty) (*empty.Empty, error)
}

// UnimplementedTouchscreenServiceServer can be embedded to have forward compatible implementations.
type UnimplementedTouchscreenServiceServer struct {
}

func (*UnimplementedTouchscreenServiceServer) NewChrome(context.Context, *empty.Empty) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method NewChrome not implemented")
}
func (*UnimplementedTouchscreenServiceServer) CloseChrome(context.Context, *empty.Empty) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CloseChrome not implemented")
}
func (*UnimplementedTouchscreenServiceServer) FindPhysicalTouchscreen(context.Context, *empty.Empty) (*FindPhysicalTouchscreenResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FindPhysicalTouchscreen not implemented")
}
func (*UnimplementedTouchscreenServiceServer) TouchscreenTap(context.Context, *empty.Empty) (*empty.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method TouchscreenTap not implemented")
}

func RegisterTouchscreenServiceServer(s *grpc.Server, srv TouchscreenServiceServer) {
	s.RegisterService(&_TouchscreenService_serviceDesc, srv)
}

func _TouchscreenService_NewChrome_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TouchscreenServiceServer).NewChrome(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.inputs.TouchscreenService/NewChrome",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TouchscreenServiceServer).NewChrome(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _TouchscreenService_CloseChrome_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TouchscreenServiceServer).CloseChrome(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.inputs.TouchscreenService/CloseChrome",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TouchscreenServiceServer).CloseChrome(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _TouchscreenService_FindPhysicalTouchscreen_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TouchscreenServiceServer).FindPhysicalTouchscreen(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.inputs.TouchscreenService/FindPhysicalTouchscreen",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TouchscreenServiceServer).FindPhysicalTouchscreen(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _TouchscreenService_TouchscreenTap_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TouchscreenServiceServer).TouchscreenTap(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.inputs.TouchscreenService/TouchscreenTap",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TouchscreenServiceServer).TouchscreenTap(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _TouchscreenService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.inputs.TouchscreenService",
	HandlerType: (*TouchscreenServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "NewChrome",
			Handler:    _TouchscreenService_NewChrome_Handler,
		},
		{
			MethodName: "CloseChrome",
			Handler:    _TouchscreenService_CloseChrome_Handler,
		},
		{
			MethodName: "FindPhysicalTouchscreen",
			Handler:    _TouchscreenService_FindPhysicalTouchscreen_Handler,
		},
		{
			MethodName: "TouchscreenTap",
			Handler:    _TouchscreenService_TouchscreenTap_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "touch_screen_service.proto",
}

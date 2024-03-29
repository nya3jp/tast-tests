// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.3
// source: adb_over_usb_service.proto

package arc

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type EnableUDCRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Enable or disable UDC on DUT. If set true, enable UDC on DUT. If set false, disable UDC on DUT.
	Enable bool `protobuf:"varint,1,opt,name=enable,proto3" json:"enable,omitempty"`
}

func (x *EnableUDCRequest) Reset() {
	*x = EnableUDCRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_adb_over_usb_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EnableUDCRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EnableUDCRequest) ProtoMessage() {}

func (x *EnableUDCRequest) ProtoReflect() protoreflect.Message {
	mi := &file_adb_over_usb_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EnableUDCRequest.ProtoReflect.Descriptor instead.
func (*EnableUDCRequest) Descriptor() ([]byte, []int) {
	return file_adb_over_usb_service_proto_rawDescGZIP(), []int{0}
}

func (x *EnableUDCRequest) GetEnable() bool {
	if x != nil {
		return x.Enable
	}
	return false
}

type EnableUDCResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Whether or not the requested value is updated successfully. If set true, UDC request executed successfully, and needs a device reboot. If set false, means no-ops or error occurred.
	UDCValueUpdated bool `protobuf:"varint,1,opt,name=UDCValueUpdated,proto3" json:"UDCValueUpdated,omitempty"`
}

func (x *EnableUDCResponse) Reset() {
	*x = EnableUDCResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_adb_over_usb_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EnableUDCResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EnableUDCResponse) ProtoMessage() {}

func (x *EnableUDCResponse) ProtoReflect() protoreflect.Message {
	mi := &file_adb_over_usb_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EnableUDCResponse.ProtoReflect.Descriptor instead.
func (*EnableUDCResponse) Descriptor() ([]byte, []int) {
	return file_adb_over_usb_service_proto_rawDescGZIP(), []int{1}
}

func (x *EnableUDCResponse) GetUDCValueUpdated() bool {
	if x != nil {
		return x.UDCValueUpdated
	}
	return false
}

var File_adb_over_usb_service_proto protoreflect.FileDescriptor

var file_adb_over_usb_service_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x61, 0x64, 0x62, 0x5f, 0x6f, 0x76, 0x65, 0x72, 0x5f, 0x75, 0x73, 0x62, 0x5f, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x74, 0x61,
	0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x61, 0x72, 0x63, 0x1a, 0x1b, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70,
	0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x2a, 0x0a, 0x10, 0x45, 0x6e, 0x61, 0x62,
	0x6c, 0x65, 0x55, 0x44, 0x43, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06,
	0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06, 0x65, 0x6e,
	0x61, 0x62, 0x6c, 0x65, 0x22, 0x3d, 0x0a, 0x11, 0x45, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x55, 0x44,
	0x43, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x28, 0x0a, 0x0f, 0x55, 0x44, 0x43,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x0f, 0x55, 0x44, 0x43, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x55, 0x70, 0x64, 0x61,
	0x74, 0x65, 0x64, 0x32, 0xb1, 0x01, 0x0a, 0x11, 0x41, 0x44, 0x42, 0x4f, 0x76, 0x65, 0x72, 0x55,
	0x53, 0x42, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x54, 0x0a, 0x0d, 0x53, 0x65, 0x74,
	0x55, 0x44, 0x43, 0x45, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x12, 0x1f, 0x2e, 0x74, 0x61, 0x73,
	0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x61, 0x72, 0x63, 0x2e, 0x45, 0x6e, 0x61, 0x62, 0x6c,
	0x65, 0x55, 0x44, 0x43, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x20, 0x2e, 0x74, 0x61,
	0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x61, 0x72, 0x63, 0x2e, 0x45, 0x6e, 0x61, 0x62,
	0x6c, 0x65, 0x55, 0x44, 0x43, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12,
	0x46, 0x0a, 0x12, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x41, 0x44, 0x42, 0x44, 0x4a, 0x6f, 0x62, 0x53,
	0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x16, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x42, 0x23, 0x5a, 0x21, 0x63, 0x68, 0x72, 0x6f, 0x6d,
	0x69, 0x75, 0x6d, 0x6f, 0x73, 0x2f, 0x74, 0x61, 0x73, 0x74, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x73, 0x2f, 0x63, 0x72, 0x6f, 0x73, 0x2f, 0x61, 0x72, 0x63, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_adb_over_usb_service_proto_rawDescOnce sync.Once
	file_adb_over_usb_service_proto_rawDescData = file_adb_over_usb_service_proto_rawDesc
)

func file_adb_over_usb_service_proto_rawDescGZIP() []byte {
	file_adb_over_usb_service_proto_rawDescOnce.Do(func() {
		file_adb_over_usb_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_adb_over_usb_service_proto_rawDescData)
	})
	return file_adb_over_usb_service_proto_rawDescData
}

var file_adb_over_usb_service_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_adb_over_usb_service_proto_goTypes = []interface{}{
	(*EnableUDCRequest)(nil),  // 0: tast.cros.arc.EnableUDCRequest
	(*EnableUDCResponse)(nil), // 1: tast.cros.arc.EnableUDCResponse
	(*emptypb.Empty)(nil),     // 2: google.protobuf.Empty
}
var file_adb_over_usb_service_proto_depIdxs = []int32{
	0, // 0: tast.cros.arc.ADBOverUSBService.SetUDCEnabled:input_type -> tast.cros.arc.EnableUDCRequest
	2, // 1: tast.cros.arc.ADBOverUSBService.CheckADBDJobStatus:input_type -> google.protobuf.Empty
	1, // 2: tast.cros.arc.ADBOverUSBService.SetUDCEnabled:output_type -> tast.cros.arc.EnableUDCResponse
	2, // 3: tast.cros.arc.ADBOverUSBService.CheckADBDJobStatus:output_type -> google.protobuf.Empty
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_adb_over_usb_service_proto_init() }
func file_adb_over_usb_service_proto_init() {
	if File_adb_over_usb_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_adb_over_usb_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EnableUDCRequest); i {
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
		file_adb_over_usb_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EnableUDCResponse); i {
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
			RawDescriptor: file_adb_over_usb_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_adb_over_usb_service_proto_goTypes,
		DependencyIndexes: file_adb_over_usb_service_proto_depIdxs,
		MessageInfos:      file_adb_over_usb_service_proto_msgTypes,
	}.Build()
	File_adb_over_usb_service_proto = out.File
	file_adb_over_usb_service_proto_rawDesc = nil
	file_adb_over_usb_service_proto_goTypes = nil
	file_adb_over_usb_service_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// ADBOverUSBServiceClient is the client API for ADBOverUSBService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type ADBOverUSBServiceClient interface {
	// Set UDC Enabled to enable or disable USB Device Controller (UDC). Return true if the requested value updated successfully. Otherwise return false.
	SetUDCEnabled(ctx context.Context, in *EnableUDCRequest, opts ...grpc.CallOption) (*EnableUDCResponse, error)
	// Check ADBD job status.
	CheckADBDJobStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type aDBOverUSBServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewADBOverUSBServiceClient(cc grpc.ClientConnInterface) ADBOverUSBServiceClient {
	return &aDBOverUSBServiceClient{cc}
}

func (c *aDBOverUSBServiceClient) SetUDCEnabled(ctx context.Context, in *EnableUDCRequest, opts ...grpc.CallOption) (*EnableUDCResponse, error) {
	out := new(EnableUDCResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.arc.ADBOverUSBService/SetUDCEnabled", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *aDBOverUSBServiceClient) CheckADBDJobStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.arc.ADBOverUSBService/CheckADBDJobStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ADBOverUSBServiceServer is the server API for ADBOverUSBService service.
type ADBOverUSBServiceServer interface {
	// Set UDC Enabled to enable or disable USB Device Controller (UDC). Return true if the requested value updated successfully. Otherwise return false.
	SetUDCEnabled(context.Context, *EnableUDCRequest) (*EnableUDCResponse, error)
	// Check ADBD job status.
	CheckADBDJobStatus(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
}

// UnimplementedADBOverUSBServiceServer can be embedded to have forward compatible implementations.
type UnimplementedADBOverUSBServiceServer struct {
}

func (*UnimplementedADBOverUSBServiceServer) SetUDCEnabled(context.Context, *EnableUDCRequest) (*EnableUDCResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetUDCEnabled not implemented")
}
func (*UnimplementedADBOverUSBServiceServer) CheckADBDJobStatus(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckADBDJobStatus not implemented")
}

func RegisterADBOverUSBServiceServer(s *grpc.Server, srv ADBOverUSBServiceServer) {
	s.RegisterService(&_ADBOverUSBService_serviceDesc, srv)
}

func _ADBOverUSBService_SetUDCEnabled_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EnableUDCRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ADBOverUSBServiceServer).SetUDCEnabled(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.arc.ADBOverUSBService/SetUDCEnabled",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ADBOverUSBServiceServer).SetUDCEnabled(ctx, req.(*EnableUDCRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ADBOverUSBService_CheckADBDJobStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ADBOverUSBServiceServer).CheckADBDJobStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.arc.ADBOverUSBService/CheckADBDJobStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ADBOverUSBServiceServer).CheckADBDJobStatus(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _ADBOverUSBService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.arc.ADBOverUSBService",
	HandlerType: (*ADBOverUSBServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SetUDCEnabled",
			Handler:    _ADBOverUSBService_SetUDCEnabled_Handler,
		},
		{
			MethodName: "CheckADBDJobStatus",
			Handler:    _ADBOverUSBService_CheckADBDJobStatus_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "adb_over_usb_service.proto",
}

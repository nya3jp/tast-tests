// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.3
// source: perf_boot_service.proto

package arc

import (
	perfpb "chromiumos/tast/common/perf/perfpb"
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	reflect "reflect"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

var File_perf_boot_service_proto protoreflect.FileDescriptor

var file_perf_boot_service_proto_rawDesc = []byte{
	0x0a, 0x17, 0x70, 0x65, 0x72, 0x66, 0x5f, 0x62, 0x6f, 0x6f, 0x74, 0x5f, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x74, 0x61, 0x73, 0x74, 0x2e,
	0x63, 0x72, 0x6f, 0x73, 0x2e, 0x61, 0x72, 0x63, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x0c, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x32, 0xa7, 0x01, 0x0a, 0x0f, 0x50, 0x65, 0x72, 0x66, 0x42, 0x6f, 0x6f, 0x74,
	0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x48, 0x0a, 0x14, 0x57, 0x61, 0x69, 0x74, 0x55,
	0x6e, 0x74, 0x69, 0x6c, 0x43, 0x50, 0x55, 0x43, 0x6f, 0x6f, 0x6c, 0x44, 0x6f, 0x77, 0x6e, 0x12,
	0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22,
	0x00, 0x12, 0x4a, 0x0a, 0x0d, 0x47, 0x65, 0x74, 0x50, 0x65, 0x72, 0x66, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x73, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x1f, 0x2e, 0x74, 0x61, 0x73,
	0x74, 0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x65, 0x72, 0x66, 0x2e, 0x70, 0x65,
	0x72, 0x66, 0x70, 0x62, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x22, 0x00, 0x42, 0x23, 0x5a,
	0x21, 0x63, 0x68, 0x72, 0x6f, 0x6d, 0x69, 0x75, 0x6d, 0x6f, 0x73, 0x2f, 0x74, 0x61, 0x73, 0x74,
	0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2f, 0x63, 0x72, 0x6f, 0x73, 0x2f, 0x61,
	0x72, 0x63, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var file_perf_boot_service_proto_goTypes = []interface{}{
	(*emptypb.Empty)(nil), // 0: google.protobuf.Empty
	(*perfpb.Values)(nil), // 1: tast.common.perf.perfpb.Values
}
var file_perf_boot_service_proto_depIdxs = []int32{
	0, // 0: tast.cros.arc.PerfBootService.WaitUntilCPUCoolDown:input_type -> google.protobuf.Empty
	0, // 1: tast.cros.arc.PerfBootService.GetPerfValues:input_type -> google.protobuf.Empty
	0, // 2: tast.cros.arc.PerfBootService.WaitUntilCPUCoolDown:output_type -> google.protobuf.Empty
	1, // 3: tast.cros.arc.PerfBootService.GetPerfValues:output_type -> tast.common.perf.perfpb.Values
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_perf_boot_service_proto_init() }
func file_perf_boot_service_proto_init() {
	if File_perf_boot_service_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_perf_boot_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_perf_boot_service_proto_goTypes,
		DependencyIndexes: file_perf_boot_service_proto_depIdxs,
	}.Build()
	File_perf_boot_service_proto = out.File
	file_perf_boot_service_proto_rawDesc = nil
	file_perf_boot_service_proto_goTypes = nil
	file_perf_boot_service_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// PerfBootServiceClient is the client API for PerfBootService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type PerfBootServiceClient interface {
	// WaitUntilCPUCoolDown internally calls power.WaitUntilCPUCoolDown on DUT
	// and waits until CPU is cooled down.
	WaitUntilCPUCoolDown(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// GetPerfValues signs in to DUT and measures Android boot performance metrics.
	GetPerfValues(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*perfpb.Values, error)
}

type perfBootServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPerfBootServiceClient(cc grpc.ClientConnInterface) PerfBootServiceClient {
	return &perfBootServiceClient{cc}
}

func (c *perfBootServiceClient) WaitUntilCPUCoolDown(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.arc.PerfBootService/WaitUntilCPUCoolDown", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *perfBootServiceClient) GetPerfValues(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*perfpb.Values, error) {
	out := new(perfpb.Values)
	err := c.cc.Invoke(ctx, "/tast.cros.arc.PerfBootService/GetPerfValues", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PerfBootServiceServer is the server API for PerfBootService service.
type PerfBootServiceServer interface {
	// WaitUntilCPUCoolDown internally calls power.WaitUntilCPUCoolDown on DUT
	// and waits until CPU is cooled down.
	WaitUntilCPUCoolDown(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	// GetPerfValues signs in to DUT and measures Android boot performance metrics.
	GetPerfValues(context.Context, *emptypb.Empty) (*perfpb.Values, error)
}

// UnimplementedPerfBootServiceServer can be embedded to have forward compatible implementations.
type UnimplementedPerfBootServiceServer struct {
}

func (*UnimplementedPerfBootServiceServer) WaitUntilCPUCoolDown(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method WaitUntilCPUCoolDown not implemented")
}
func (*UnimplementedPerfBootServiceServer) GetPerfValues(context.Context, *emptypb.Empty) (*perfpb.Values, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPerfValues not implemented")
}

func RegisterPerfBootServiceServer(s *grpc.Server, srv PerfBootServiceServer) {
	s.RegisterService(&_PerfBootService_serviceDesc, srv)
}

func _PerfBootService_WaitUntilCPUCoolDown_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PerfBootServiceServer).WaitUntilCPUCoolDown(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.arc.PerfBootService/WaitUntilCPUCoolDown",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PerfBootServiceServer).WaitUntilCPUCoolDown(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _PerfBootService_GetPerfValues_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PerfBootServiceServer).GetPerfValues(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.arc.PerfBootService/GetPerfValues",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PerfBootServiceServer).GetPerfValues(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _PerfBootService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.arc.PerfBootService",
	HandlerType: (*PerfBootServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "WaitUntilCPUCoolDown",
			Handler:    _PerfBootService_WaitUntilCPUCoolDown_Handler,
		},
		{
			MethodName: "GetPerfValues",
			Handler:    _PerfBootService_GetPerfValues_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "perf_boot_service.proto",
}

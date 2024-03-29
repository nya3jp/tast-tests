// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.3
// source: rollback_service.proto

package autoupdate

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

// Deprecated: Do not use.
type SetUpPskResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Guid string `protobuf:"bytes,1,opt,name=guid,proto3" json:"guid,omitempty"`
}

func (x *SetUpPskResponse) Reset() {
	*x = SetUpPskResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rollback_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetUpPskResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetUpPskResponse) ProtoMessage() {}

func (x *SetUpPskResponse) ProtoReflect() protoreflect.Message {
	mi := &file_rollback_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetUpPskResponse.ProtoReflect.Descriptor instead.
func (*SetUpPskResponse) Descriptor() ([]byte, []int) {
	return file_rollback_service_proto_rawDescGZIP(), []int{0}
}

func (x *SetUpPskResponse) GetGuid() string {
	if x != nil {
		return x.Guid
	}
	return ""
}

type NetworkInformation struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Guid string `protobuf:"bytes,1,opt,name=guid,proto3" json:"guid,omitempty"`
}

func (x *NetworkInformation) Reset() {
	*x = NetworkInformation{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rollback_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NetworkInformation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkInformation) ProtoMessage() {}

func (x *NetworkInformation) ProtoReflect() protoreflect.Message {
	mi := &file_rollback_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkInformation.ProtoReflect.Descriptor instead.
func (*NetworkInformation) Descriptor() ([]byte, []int) {
	return file_rollback_service_proto_rawDescGZIP(), []int{1}
}

func (x *NetworkInformation) GetGuid() string {
	if x != nil {
		return x.Guid
	}
	return ""
}

type SetUpNetworksRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *SetUpNetworksRequest) Reset() {
	*x = SetUpNetworksRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rollback_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetUpNetworksRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetUpNetworksRequest) ProtoMessage() {}

func (x *SetUpNetworksRequest) ProtoReflect() protoreflect.Message {
	mi := &file_rollback_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetUpNetworksRequest.ProtoReflect.Descriptor instead.
func (*SetUpNetworksRequest) Descriptor() ([]byte, []int) {
	return file_rollback_service_proto_rawDescGZIP(), []int{2}
}

type SetUpNetworksResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Networks []*NetworkInformation `protobuf:"bytes,1,rep,name=networks,proto3" json:"networks,omitempty"`
}

func (x *SetUpNetworksResponse) Reset() {
	*x = SetUpNetworksResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rollback_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetUpNetworksResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetUpNetworksResponse) ProtoMessage() {}

func (x *SetUpNetworksResponse) ProtoReflect() protoreflect.Message {
	mi := &file_rollback_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetUpNetworksResponse.ProtoReflect.Descriptor instead.
func (*SetUpNetworksResponse) Descriptor() ([]byte, []int) {
	return file_rollback_service_proto_rawDescGZIP(), []int{3}
}

func (x *SetUpNetworksResponse) GetNetworks() []*NetworkInformation {
	if x != nil {
		return x.Networks
	}
	return nil
}

// VerifyRollbackRequest needs to contain the unchanged NetworkInformation from
// SetUpNetworksResponse.
type VerifyRollbackRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Networks []*NetworkInformation `protobuf:"bytes,2,rep,name=networks,proto3" json:"networks,omitempty"`
}

func (x *VerifyRollbackRequest) Reset() {
	*x = VerifyRollbackRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rollback_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VerifyRollbackRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VerifyRollbackRequest) ProtoMessage() {}

func (x *VerifyRollbackRequest) ProtoReflect() protoreflect.Message {
	mi := &file_rollback_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VerifyRollbackRequest.ProtoReflect.Descriptor instead.
func (*VerifyRollbackRequest) Descriptor() ([]byte, []int) {
	return file_rollback_service_proto_rawDescGZIP(), []int{4}
}

func (x *VerifyRollbackRequest) GetNetworks() []*NetworkInformation {
	if x != nil {
		return x.Networks
	}
	return nil
}

type VerifyRollbackResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Successful bool `protobuf:"varint,1,opt,name=successful,proto3" json:"successful,omitempty"`
	// It provides details about the failure or further information about the
	// success that should be logged for reference.
	VerificationDetails string `protobuf:"bytes,2,opt,name=verification_details,json=verificationDetails,proto3" json:"verification_details,omitempty"`
}

func (x *VerifyRollbackResponse) Reset() {
	*x = VerifyRollbackResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_rollback_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VerifyRollbackResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VerifyRollbackResponse) ProtoMessage() {}

func (x *VerifyRollbackResponse) ProtoReflect() protoreflect.Message {
	mi := &file_rollback_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VerifyRollbackResponse.ProtoReflect.Descriptor instead.
func (*VerifyRollbackResponse) Descriptor() ([]byte, []int) {
	return file_rollback_service_proto_rawDescGZIP(), []int{5}
}

func (x *VerifyRollbackResponse) GetSuccessful() bool {
	if x != nil {
		return x.Successful
	}
	return false
}

func (x *VerifyRollbackResponse) GetVerificationDetails() string {
	if x != nil {
		return x.VerificationDetails
	}
	return ""
}

var File_rollback_service_proto protoreflect.FileDescriptor

var file_rollback_service_proto_rawDesc = []byte{
	0x0a, 0x16, 0x72, 0x6f, 0x6c, 0x6c, 0x62, 0x61, 0x63, 0x6b, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x14, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63,
	0x72, 0x6f, 0x73, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x1a, 0x1b,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x2a, 0x0a, 0x10, 0x53,
	0x65, 0x74, 0x55, 0x70, 0x50, 0x73, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x12, 0x0a, 0x04, 0x67, 0x75, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x67,
	0x75, 0x69, 0x64, 0x3a, 0x02, 0x18, 0x01, 0x22, 0x28, 0x0a, 0x12, 0x4e, 0x65, 0x74, 0x77, 0x6f,
	0x72, 0x6b, 0x49, 0x6e, 0x66, 0x6f, 0x72, 0x6d, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a,
	0x04, 0x67, 0x75, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x67, 0x75, 0x69,
	0x64, 0x22, 0x16, 0x0a, 0x14, 0x53, 0x65, 0x74, 0x55, 0x70, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72,
	0x6b, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x5d, 0x0a, 0x15, 0x53, 0x65, 0x74,
	0x55, 0x70, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x44, 0x0a, 0x08, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x28, 0x2e, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73,
	0x2e, 0x61, 0x75, 0x74, 0x6f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x4e, 0x65, 0x74, 0x77,
	0x6f, 0x72, 0x6b, 0x49, 0x6e, 0x66, 0x6f, 0x72, 0x6d, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x08,
	0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x22, 0x63, 0x0a, 0x15, 0x56, 0x65, 0x72, 0x69,
	0x66, 0x79, 0x52, 0x6f, 0x6c, 0x6c, 0x62, 0x61, 0x63, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x44, 0x0a, 0x08, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x18, 0x02, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x28, 0x2e, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e,
	0x61, 0x75, 0x74, 0x6f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f,
	0x72, 0x6b, 0x49, 0x6e, 0x66, 0x6f, 0x72, 0x6d, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x08, 0x6e,
	0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x4a, 0x04, 0x08, 0x01, 0x10, 0x02, 0x22, 0x6b, 0x0a,
	0x16, 0x56, 0x65, 0x72, 0x69, 0x66, 0x79, 0x52, 0x6f, 0x6c, 0x6c, 0x62, 0x61, 0x63, 0x6b, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x75, 0x63, 0x63, 0x65,
	0x73, 0x73, 0x66, 0x75, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0a, 0x73, 0x75, 0x63,
	0x63, 0x65, 0x73, 0x73, 0x66, 0x75, 0x6c, 0x12, 0x31, 0x0a, 0x14, 0x76, 0x65, 0x72, 0x69, 0x66,
	0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x13, 0x76, 0x65, 0x72, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x32, 0xc0, 0x02, 0x0a, 0x0f, 0x52,
	0x6f, 0x6c, 0x6c, 0x62, 0x61, 0x63, 0x6b, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x56,
	0x0a, 0x0f, 0x53, 0x65, 0x74, 0x55, 0x70, 0x50, 0x73, 0x6b, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72,
	0x6b, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x26, 0x2e, 0x74, 0x61, 0x73, 0x74,
	0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65,
	0x2e, 0x53, 0x65, 0x74, 0x55, 0x70, 0x50, 0x73, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x03, 0x88, 0x02, 0x01, 0x12, 0x68, 0x0a, 0x0d, 0x53, 0x65, 0x74, 0x55, 0x70, 0x4e,
	0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x12, 0x2a, 0x2e, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63,
	0x72, 0x6f, 0x73, 0x2e, 0x61, 0x75, 0x74, 0x6f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x53,
	0x65, 0x74, 0x55, 0x70, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x2b, 0x2e, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e,
	0x61, 0x75, 0x74, 0x6f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x53, 0x65, 0x74, 0x55, 0x70,
	0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x6b, 0x0a, 0x0e, 0x56, 0x65, 0x72, 0x69, 0x66, 0x79, 0x52, 0x6f, 0x6c, 0x6c, 0x62, 0x61,
	0x63, 0x6b, 0x12, 0x2b, 0x2e, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x61,
	0x75, 0x74, 0x6f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x56, 0x65, 0x72, 0x69, 0x66, 0x79,
	0x52, 0x6f, 0x6c, 0x6c, 0x62, 0x61, 0x63, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x2c, 0x2e, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x61, 0x75, 0x74, 0x6f,
	0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x56, 0x65, 0x72, 0x69, 0x66, 0x79, 0x52, 0x6f, 0x6c,
	0x6c, 0x62, 0x61, 0x63, 0x6b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x2a, 0x5a,
	0x28, 0x63, 0x68, 0x72, 0x6f, 0x6d, 0x69, 0x75, 0x6d, 0x6f, 0x73, 0x2f, 0x74, 0x61, 0x73, 0x74,
	0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2f, 0x63, 0x72, 0x6f, 0x73, 0x2f, 0x61,
	0x75, 0x74, 0x6f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_rollback_service_proto_rawDescOnce sync.Once
	file_rollback_service_proto_rawDescData = file_rollback_service_proto_rawDesc
)

func file_rollback_service_proto_rawDescGZIP() []byte {
	file_rollback_service_proto_rawDescOnce.Do(func() {
		file_rollback_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_rollback_service_proto_rawDescData)
	})
	return file_rollback_service_proto_rawDescData
}

var file_rollback_service_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_rollback_service_proto_goTypes = []interface{}{
	(*SetUpPskResponse)(nil),       // 0: tast.cros.autoupdate.SetUpPskResponse
	(*NetworkInformation)(nil),     // 1: tast.cros.autoupdate.NetworkInformation
	(*SetUpNetworksRequest)(nil),   // 2: tast.cros.autoupdate.SetUpNetworksRequest
	(*SetUpNetworksResponse)(nil),  // 3: tast.cros.autoupdate.SetUpNetworksResponse
	(*VerifyRollbackRequest)(nil),  // 4: tast.cros.autoupdate.VerifyRollbackRequest
	(*VerifyRollbackResponse)(nil), // 5: tast.cros.autoupdate.VerifyRollbackResponse
	(*emptypb.Empty)(nil),          // 6: google.protobuf.Empty
}
var file_rollback_service_proto_depIdxs = []int32{
	1, // 0: tast.cros.autoupdate.SetUpNetworksResponse.networks:type_name -> tast.cros.autoupdate.NetworkInformation
	1, // 1: tast.cros.autoupdate.VerifyRollbackRequest.networks:type_name -> tast.cros.autoupdate.NetworkInformation
	6, // 2: tast.cros.autoupdate.RollbackService.SetUpPskNetwork:input_type -> google.protobuf.Empty
	2, // 3: tast.cros.autoupdate.RollbackService.SetUpNetworks:input_type -> tast.cros.autoupdate.SetUpNetworksRequest
	4, // 4: tast.cros.autoupdate.RollbackService.VerifyRollback:input_type -> tast.cros.autoupdate.VerifyRollbackRequest
	0, // 5: tast.cros.autoupdate.RollbackService.SetUpPskNetwork:output_type -> tast.cros.autoupdate.SetUpPskResponse
	3, // 6: tast.cros.autoupdate.RollbackService.SetUpNetworks:output_type -> tast.cros.autoupdate.SetUpNetworksResponse
	5, // 7: tast.cros.autoupdate.RollbackService.VerifyRollback:output_type -> tast.cros.autoupdate.VerifyRollbackResponse
	5, // [5:8] is the sub-list for method output_type
	2, // [2:5] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_rollback_service_proto_init() }
func file_rollback_service_proto_init() {
	if File_rollback_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_rollback_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetUpPskResponse); i {
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
		file_rollback_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NetworkInformation); i {
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
		file_rollback_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetUpNetworksRequest); i {
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
		file_rollback_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetUpNetworksResponse); i {
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
		file_rollback_service_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VerifyRollbackRequest); i {
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
		file_rollback_service_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VerifyRollbackResponse); i {
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
			RawDescriptor: file_rollback_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_rollback_service_proto_goTypes,
		DependencyIndexes: file_rollback_service_proto_depIdxs,
		MessageInfos:      file_rollback_service_proto_msgTypes,
	}.Build()
	File_rollback_service_proto = out.File
	file_rollback_service_proto_rawDesc = nil
	file_rollback_service_proto_goTypes = nil
	file_rollback_service_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// RollbackServiceClient is the client API for RollbackService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RollbackServiceClient interface {
	// Deprecated: Do not use.
	// SetUpPskNetwork is deprecated and replaced by SetUpNetworks.
	SetUpPskNetwork(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SetUpPskResponse, error)
	// SetUpNetworks sets various networks supported by rollback automatically.
	SetUpNetworks(ctx context.Context, in *SetUpNetworksRequest, opts ...grpc.CallOption) (*SetUpNetworksResponse, error)
	// VerifyRollback verifies that oobe ends up on the enrollment screen after
	// rollback and that the networks provided still exists.
	VerifyRollback(ctx context.Context, in *VerifyRollbackRequest, opts ...grpc.CallOption) (*VerifyRollbackResponse, error)
}

type rollbackServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewRollbackServiceClient(cc grpc.ClientConnInterface) RollbackServiceClient {
	return &rollbackServiceClient{cc}
}

// Deprecated: Do not use.
func (c *rollbackServiceClient) SetUpPskNetwork(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SetUpPskResponse, error) {
	out := new(SetUpPskResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.autoupdate.RollbackService/SetUpPskNetwork", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rollbackServiceClient) SetUpNetworks(ctx context.Context, in *SetUpNetworksRequest, opts ...grpc.CallOption) (*SetUpNetworksResponse, error) {
	out := new(SetUpNetworksResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.autoupdate.RollbackService/SetUpNetworks", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *rollbackServiceClient) VerifyRollback(ctx context.Context, in *VerifyRollbackRequest, opts ...grpc.CallOption) (*VerifyRollbackResponse, error) {
	out := new(VerifyRollbackResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.autoupdate.RollbackService/VerifyRollback", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RollbackServiceServer is the server API for RollbackService service.
type RollbackServiceServer interface {
	// Deprecated: Do not use.
	// SetUpPskNetwork is deprecated and replaced by SetUpNetworks.
	SetUpPskNetwork(context.Context, *emptypb.Empty) (*SetUpPskResponse, error)
	// SetUpNetworks sets various networks supported by rollback automatically.
	SetUpNetworks(context.Context, *SetUpNetworksRequest) (*SetUpNetworksResponse, error)
	// VerifyRollback verifies that oobe ends up on the enrollment screen after
	// rollback and that the networks provided still exists.
	VerifyRollback(context.Context, *VerifyRollbackRequest) (*VerifyRollbackResponse, error)
}

// UnimplementedRollbackServiceServer can be embedded to have forward compatible implementations.
type UnimplementedRollbackServiceServer struct {
}

func (*UnimplementedRollbackServiceServer) SetUpPskNetwork(context.Context, *emptypb.Empty) (*SetUpPskResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetUpPskNetwork not implemented")
}
func (*UnimplementedRollbackServiceServer) SetUpNetworks(context.Context, *SetUpNetworksRequest) (*SetUpNetworksResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetUpNetworks not implemented")
}
func (*UnimplementedRollbackServiceServer) VerifyRollback(context.Context, *VerifyRollbackRequest) (*VerifyRollbackResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method VerifyRollback not implemented")
}

func RegisterRollbackServiceServer(s *grpc.Server, srv RollbackServiceServer) {
	s.RegisterService(&_RollbackService_serviceDesc, srv)
}

func _RollbackService_SetUpPskNetwork_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RollbackServiceServer).SetUpPskNetwork(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.autoupdate.RollbackService/SetUpPskNetwork",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RollbackServiceServer).SetUpPskNetwork(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _RollbackService_SetUpNetworks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetUpNetworksRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RollbackServiceServer).SetUpNetworks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.autoupdate.RollbackService/SetUpNetworks",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RollbackServiceServer).SetUpNetworks(ctx, req.(*SetUpNetworksRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _RollbackService_VerifyRollback_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(VerifyRollbackRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RollbackServiceServer).VerifyRollback(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.autoupdate.RollbackService/VerifyRollback",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RollbackServiceServer).VerifyRollback(ctx, req.(*VerifyRollbackRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _RollbackService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.autoupdate.RollbackService",
	HandlerType: (*RollbackServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SetUpPskNetwork",
			Handler:    _RollbackService_SetUpPskNetwork_Handler,
		},
		{
			MethodName: "SetUpNetworks",
			Handler:    _RollbackService_SetUpNetworks_Handler,
		},
		{
			MethodName: "VerifyRollback",
			Handler:    _RollbackService_VerifyRollback_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "rollback_service.proto",
}

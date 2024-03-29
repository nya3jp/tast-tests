// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.19.3
// source: hps_setting_service.proto

package hps

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type StartUIWithCustomScreenPrivacySettingRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Setting string `protobuf:"bytes,1,opt,name=setting,proto3" json:"setting,omitempty"` // Option being "Lock on Leave" or "Viewing protection".
	Enable  bool   `protobuf:"varint,2,opt,name=enable,proto3" json:"enable,omitempty"`
}

func (x *StartUIWithCustomScreenPrivacySettingRequest) Reset() {
	*x = StartUIWithCustomScreenPrivacySettingRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hps_setting_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StartUIWithCustomScreenPrivacySettingRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StartUIWithCustomScreenPrivacySettingRequest) ProtoMessage() {}

func (x *StartUIWithCustomScreenPrivacySettingRequest) ProtoReflect() protoreflect.Message {
	mi := &file_hps_setting_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StartUIWithCustomScreenPrivacySettingRequest.ProtoReflect.Descriptor instead.
func (*StartUIWithCustomScreenPrivacySettingRequest) Descriptor() ([]byte, []int) {
	return file_hps_setting_service_proto_rawDescGZIP(), []int{0}
}

func (x *StartUIWithCustomScreenPrivacySettingRequest) GetSetting() string {
	if x != nil {
		return x.Setting
	}
	return ""
}

func (x *StartUIWithCustomScreenPrivacySettingRequest) GetEnable() bool {
	if x != nil {
		return x.Enable
	}
	return false
}

// Note that the HPS D-Bus method names use the older terminology:
// "sense" = presence detection, for "Lock on leave" functionality
// "notify" = second person detection, for "Viewing protection" functionality
type WaitForHpsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	WaitForSense  bool `protobuf:"varint,1,opt,name=wait_for_sense,json=waitForSense,proto3" json:"wait_for_sense,omitempty"`    // Also wait for HPS to enable feature 0 ("sense")
	WaitForNotify bool `protobuf:"varint,2,opt,name=wait_for_notify,json=waitForNotify,proto3" json:"wait_for_notify,omitempty"` // Also wait for HPS to enable feature 1 ("notify")
}

func (x *WaitForHpsRequest) Reset() {
	*x = WaitForHpsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hps_setting_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WaitForHpsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WaitForHpsRequest) ProtoMessage() {}

func (x *WaitForHpsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_hps_setting_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WaitForHpsRequest.ProtoReflect.Descriptor instead.
func (*WaitForHpsRequest) Descriptor() ([]byte, []int) {
	return file_hps_setting_service_proto_rawDescGZIP(), []int{1}
}

func (x *WaitForHpsRequest) GetWaitForSense() bool {
	if x != nil {
		return x.WaitForSense
	}
	return false
}

func (x *WaitForHpsRequest) GetWaitForNotify() bool {
	if x != nil {
		return x.WaitForNotify
	}
	return false
}

type RetrieveDimMetricsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DimDelay       *durationpb.Duration `protobuf:"bytes,1,opt,name=dimDelay,proto3" json:"dimDelay,omitempty"`
	ScreenOffDelay *durationpb.Duration `protobuf:"bytes,2,opt,name=screenOffDelay,proto3" json:"screenOffDelay,omitempty"`
	LockDelay      *durationpb.Duration `protobuf:"bytes,3,opt,name=lockDelay,proto3" json:"lockDelay,omitempty"`
}

func (x *RetrieveDimMetricsResponse) Reset() {
	*x = RetrieveDimMetricsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hps_setting_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RetrieveDimMetricsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RetrieveDimMetricsResponse) ProtoMessage() {}

func (x *RetrieveDimMetricsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_hps_setting_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RetrieveDimMetricsResponse.ProtoReflect.Descriptor instead.
func (*RetrieveDimMetricsResponse) Descriptor() ([]byte, []int) {
	return file_hps_setting_service_proto_rawDescGZIP(), []int{2}
}

func (x *RetrieveDimMetricsResponse) GetDimDelay() *durationpb.Duration {
	if x != nil {
		return x.DimDelay
	}
	return nil
}

func (x *RetrieveDimMetricsResponse) GetScreenOffDelay() *durationpb.Duration {
	if x != nil {
		return x.ScreenOffDelay
	}
	return nil
}

func (x *RetrieveDimMetricsResponse) GetLockDelay() *durationpb.Duration {
	if x != nil {
		return x.LockDelay
	}
	return nil
}

type HpsSenseSignalResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RawValue string `protobuf:"bytes,1,opt,name=raw_value,json=rawValue,proto3" json:"raw_value,omitempty"`
}

func (x *HpsSenseSignalResponse) Reset() {
	*x = HpsSenseSignalResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_hps_setting_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HpsSenseSignalResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HpsSenseSignalResponse) ProtoMessage() {}

func (x *HpsSenseSignalResponse) ProtoReflect() protoreflect.Message {
	mi := &file_hps_setting_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HpsSenseSignalResponse.ProtoReflect.Descriptor instead.
func (*HpsSenseSignalResponse) Descriptor() ([]byte, []int) {
	return file_hps_setting_service_proto_rawDescGZIP(), []int{3}
}

func (x *HpsSenseSignalResponse) GetRawValue() string {
	if x != nil {
		return x.RawValue
	}
	return ""
}

var File_hps_setting_service_proto protoreflect.FileDescriptor

var file_hps_setting_service_proto_rawDesc = []byte{
	0x0a, 0x19, 0x68, 0x70, 0x73, 0x5f, 0x73, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x5f, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x74, 0x61, 0x73,
	0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x68, 0x70, 0x73, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74,
	0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x77, 0x72, 0x61, 0x70, 0x70, 0x65, 0x72,
	0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x60, 0x0a, 0x2c, 0x53, 0x74, 0x61, 0x72, 0x74,
	0x55, 0x49, 0x57, 0x69, 0x74, 0x68, 0x43, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x53, 0x63, 0x72, 0x65,
	0x65, 0x6e, 0x50, 0x72, 0x69, 0x76, 0x61, 0x63, 0x79, 0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x65, 0x74, 0x74, 0x69,
	0x6e, 0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x73, 0x65, 0x74, 0x74, 0x69, 0x6e,
	0x67, 0x12, 0x16, 0x0a, 0x06, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x06, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x22, 0x61, 0x0a, 0x11, 0x57, 0x61, 0x69,
	0x74, 0x46, 0x6f, 0x72, 0x48, 0x70, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x24,
	0x0a, 0x0e, 0x77, 0x61, 0x69, 0x74, 0x5f, 0x66, 0x6f, 0x72, 0x5f, 0x73, 0x65, 0x6e, 0x73, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0c, 0x77, 0x61, 0x69, 0x74, 0x46, 0x6f, 0x72, 0x53,
	0x65, 0x6e, 0x73, 0x65, 0x12, 0x26, 0x0a, 0x0f, 0x77, 0x61, 0x69, 0x74, 0x5f, 0x66, 0x6f, 0x72,
	0x5f, 0x6e, 0x6f, 0x74, 0x69, 0x66, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0d, 0x77,
	0x61, 0x69, 0x74, 0x46, 0x6f, 0x72, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x79, 0x22, 0xcf, 0x01, 0x0a,
	0x1a, 0x52, 0x65, 0x74, 0x72, 0x69, 0x65, 0x76, 0x65, 0x44, 0x69, 0x6d, 0x4d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x35, 0x0a, 0x08, 0x64,
	0x69, 0x6d, 0x44, 0x65, 0x6c, 0x61, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x08, 0x64, 0x69, 0x6d, 0x44, 0x65, 0x6c,
	0x61, 0x79, 0x12, 0x41, 0x0a, 0x0e, 0x73, 0x63, 0x72, 0x65, 0x65, 0x6e, 0x4f, 0x66, 0x66, 0x44,
	0x65, 0x6c, 0x61, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0e, 0x73, 0x63, 0x72, 0x65, 0x65, 0x6e, 0x4f, 0x66, 0x66,
	0x44, 0x65, 0x6c, 0x61, 0x79, 0x12, 0x37, 0x0a, 0x09, 0x6c, 0x6f, 0x63, 0x6b, 0x44, 0x65, 0x6c,
	0x61, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x52, 0x09, 0x6c, 0x6f, 0x63, 0x6b, 0x44, 0x65, 0x6c, 0x61, 0x79, 0x22, 0x35,
	0x0a, 0x16, 0x48, 0x70, 0x73, 0x53, 0x65, 0x6e, 0x73, 0x65, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x6c,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x72, 0x61, 0x77, 0x5f,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x61, 0x77,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x32, 0xeb, 0x04, 0x0a, 0x0a, 0x48, 0x70, 0x73, 0x53, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x12, 0x7e, 0x0a, 0x25, 0x53, 0x74, 0x61, 0x72, 0x74, 0x55, 0x49, 0x57,
	0x69, 0x74, 0x68, 0x43, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x53, 0x63, 0x72, 0x65, 0x65, 0x6e, 0x50,
	0x72, 0x69, 0x76, 0x61, 0x63, 0x79, 0x53, 0x65, 0x74, 0x74, 0x69, 0x6e, 0x67, 0x12, 0x3b, 0x2e,
	0x74, 0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x68, 0x70, 0x73, 0x2e, 0x53, 0x74,
	0x61, 0x72, 0x74, 0x55, 0x49, 0x57, 0x69, 0x74, 0x68, 0x43, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x53,
	0x63, 0x72, 0x65, 0x65, 0x6e, 0x50, 0x72, 0x69, 0x76, 0x61, 0x63, 0x79, 0x53, 0x65, 0x74, 0x74,
	0x69, 0x6e, 0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70,
	0x74, 0x79, 0x22, 0x00, 0x12, 0x48, 0x0a, 0x0a, 0x57, 0x61, 0x69, 0x74, 0x46, 0x6f, 0x72, 0x48,
	0x70, 0x73, 0x12, 0x20, 0x2e, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x68,
	0x70, 0x73, 0x2e, 0x57, 0x61, 0x69, 0x74, 0x46, 0x6f, 0x72, 0x48, 0x70, 0x73, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x46,
	0x0a, 0x12, 0x43, 0x68, 0x65, 0x63, 0x6b, 0x46, 0x6f, 0x72, 0x4c, 0x6f, 0x63, 0x6b, 0x53, 0x63,
	0x72, 0x65, 0x65, 0x6e, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x16, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45,
	0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x48, 0x0a, 0x14, 0x4f, 0x70, 0x65, 0x6e, 0x48, 0x50,
	0x53, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x73, 0x50, 0x61, 0x67, 0x65, 0x12, 0x16,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00,
	0x12, 0x5d, 0x0a, 0x12, 0x52, 0x65, 0x74, 0x72, 0x69, 0x65, 0x76, 0x65, 0x44, 0x69, 0x6d, 0x4d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x12, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x42, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x1a, 0x29, 0x2e, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x68,
	0x70, 0x73, 0x2e, 0x52, 0x65, 0x74, 0x72, 0x69, 0x65, 0x76, 0x65, 0x44, 0x69, 0x6d, 0x4d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12,
	0x59, 0x0a, 0x16, 0x52, 0x65, 0x74, 0x72, 0x69, 0x65, 0x76, 0x65, 0x48, 0x70, 0x73, 0x53, 0x65,
	0x6e, 0x73, 0x65, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x6c, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74,
	0x79, 0x1a, 0x25, 0x2e, 0x74, 0x61, 0x73, 0x74, 0x2e, 0x63, 0x72, 0x6f, 0x73, 0x2e, 0x68, 0x70,
	0x73, 0x2e, 0x48, 0x70, 0x73, 0x53, 0x65, 0x6e, 0x73, 0x65, 0x53, 0x69, 0x67, 0x6e, 0x61, 0x6c,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x47, 0x0a, 0x0f, 0x43, 0x68,
	0x65, 0x63, 0x6b, 0x53, 0x50, 0x41, 0x45, 0x79, 0x65, 0x49, 0x63, 0x6f, 0x6e, 0x12, 0x16, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x42, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x22, 0x00, 0x42, 0x23, 0x5a, 0x21, 0x63, 0x68, 0x72, 0x6f, 0x6d, 0x69, 0x75, 0x6d, 0x6f,
	0x73, 0x2f, 0x74, 0x61, 0x73, 0x74, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2f,
	0x63, 0x72, 0x6f, 0x73, 0x2f, 0x68, 0x70, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_hps_setting_service_proto_rawDescOnce sync.Once
	file_hps_setting_service_proto_rawDescData = file_hps_setting_service_proto_rawDesc
)

func file_hps_setting_service_proto_rawDescGZIP() []byte {
	file_hps_setting_service_proto_rawDescOnce.Do(func() {
		file_hps_setting_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_hps_setting_service_proto_rawDescData)
	})
	return file_hps_setting_service_proto_rawDescData
}

var file_hps_setting_service_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_hps_setting_service_proto_goTypes = []interface{}{
	(*StartUIWithCustomScreenPrivacySettingRequest)(nil), // 0: tast.cros.hps.StartUIWithCustomScreenPrivacySettingRequest
	(*WaitForHpsRequest)(nil),                            // 1: tast.cros.hps.WaitForHpsRequest
	(*RetrieveDimMetricsResponse)(nil),                   // 2: tast.cros.hps.RetrieveDimMetricsResponse
	(*HpsSenseSignalResponse)(nil),                       // 3: tast.cros.hps.HpsSenseSignalResponse
	(*durationpb.Duration)(nil),                          // 4: google.protobuf.Duration
	(*emptypb.Empty)(nil),                                // 5: google.protobuf.Empty
	(*wrapperspb.BoolValue)(nil),                         // 6: google.protobuf.BoolValue
}
var file_hps_setting_service_proto_depIdxs = []int32{
	4,  // 0: tast.cros.hps.RetrieveDimMetricsResponse.dimDelay:type_name -> google.protobuf.Duration
	4,  // 1: tast.cros.hps.RetrieveDimMetricsResponse.screenOffDelay:type_name -> google.protobuf.Duration
	4,  // 2: tast.cros.hps.RetrieveDimMetricsResponse.lockDelay:type_name -> google.protobuf.Duration
	0,  // 3: tast.cros.hps.HpsService.StartUIWithCustomScreenPrivacySetting:input_type -> tast.cros.hps.StartUIWithCustomScreenPrivacySettingRequest
	1,  // 4: tast.cros.hps.HpsService.WaitForHps:input_type -> tast.cros.hps.WaitForHpsRequest
	5,  // 5: tast.cros.hps.HpsService.CheckForLockScreen:input_type -> google.protobuf.Empty
	5,  // 6: tast.cros.hps.HpsService.OpenHPSInternalsPage:input_type -> google.protobuf.Empty
	6,  // 7: tast.cros.hps.HpsService.RetrieveDimMetrics:input_type -> google.protobuf.BoolValue
	5,  // 8: tast.cros.hps.HpsService.RetrieveHpsSenseSignal:input_type -> google.protobuf.Empty
	5,  // 9: tast.cros.hps.HpsService.CheckSPAEyeIcon:input_type -> google.protobuf.Empty
	5,  // 10: tast.cros.hps.HpsService.StartUIWithCustomScreenPrivacySetting:output_type -> google.protobuf.Empty
	5,  // 11: tast.cros.hps.HpsService.WaitForHps:output_type -> google.protobuf.Empty
	5,  // 12: tast.cros.hps.HpsService.CheckForLockScreen:output_type -> google.protobuf.Empty
	5,  // 13: tast.cros.hps.HpsService.OpenHPSInternalsPage:output_type -> google.protobuf.Empty
	2,  // 14: tast.cros.hps.HpsService.RetrieveDimMetrics:output_type -> tast.cros.hps.RetrieveDimMetricsResponse
	3,  // 15: tast.cros.hps.HpsService.RetrieveHpsSenseSignal:output_type -> tast.cros.hps.HpsSenseSignalResponse
	6,  // 16: tast.cros.hps.HpsService.CheckSPAEyeIcon:output_type -> google.protobuf.BoolValue
	10, // [10:17] is the sub-list for method output_type
	3,  // [3:10] is the sub-list for method input_type
	3,  // [3:3] is the sub-list for extension type_name
	3,  // [3:3] is the sub-list for extension extendee
	0,  // [0:3] is the sub-list for field type_name
}

func init() { file_hps_setting_service_proto_init() }
func file_hps_setting_service_proto_init() {
	if File_hps_setting_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_hps_setting_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StartUIWithCustomScreenPrivacySettingRequest); i {
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
		file_hps_setting_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WaitForHpsRequest); i {
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
		file_hps_setting_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RetrieveDimMetricsResponse); i {
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
		file_hps_setting_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HpsSenseSignalResponse); i {
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
			RawDescriptor: file_hps_setting_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_hps_setting_service_proto_goTypes,
		DependencyIndexes: file_hps_setting_service_proto_depIdxs,
		MessageInfos:      file_hps_setting_service_proto_msgTypes,
	}.Build()
	File_hps_setting_service_proto = out.File
	file_hps_setting_service_proto_rawDesc = nil
	file_hps_setting_service_proto_goTypes = nil
	file_hps_setting_service_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// HpsServiceClient is the client API for HpsService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type HpsServiceClient interface {
	// StartUIWithCustomScreenPrivacySetting changes the settings in screen privacy accordingly.
	StartUIWithCustomScreenPrivacySetting(ctx context.Context, in *StartUIWithCustomScreenPrivacySettingRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// WaitForHps waits for hpsd to be ready, and optionally to finish enabling the requested features.
	// This includes booting the HPS peripheral and potentially flashing its firmware.
	WaitForHps(ctx context.Context, in *WaitForHpsRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// CheckForLockScreen checks if the screen is at lock status.
	CheckForLockScreen(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// OpenHPSInternalPage opens hps-internal page for debugging purpose.
	OpenHPSInternalsPage(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	// DimMetrics gets the quick dim/lock delays after the lol is enabled/disabled.
	RetrieveDimMetrics(ctx context.Context, in *wrapperspb.BoolValue, opts ...grpc.CallOption) (*RetrieveDimMetricsResponse, error)
	// RetrieveHpsSenseSignal gets current HpsSenseSignal from powerd.
	RetrieveHpsSenseSignal(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HpsSenseSignalResponse, error)
	// CheckSPAEyeIcon checks if the eye icon is at the right bottom side of the screen when there is spa alert.
	CheckSPAEyeIcon(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*wrapperspb.BoolValue, error)
}

type hpsServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewHpsServiceClient(cc grpc.ClientConnInterface) HpsServiceClient {
	return &hpsServiceClient{cc}
}

func (c *hpsServiceClient) StartUIWithCustomScreenPrivacySetting(ctx context.Context, in *StartUIWithCustomScreenPrivacySettingRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.hps.HpsService/StartUIWithCustomScreenPrivacySetting", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hpsServiceClient) WaitForHps(ctx context.Context, in *WaitForHpsRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.hps.HpsService/WaitForHps", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hpsServiceClient) CheckForLockScreen(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.hps.HpsService/CheckForLockScreen", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hpsServiceClient) OpenHPSInternalsPage(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/tast.cros.hps.HpsService/OpenHPSInternalsPage", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hpsServiceClient) RetrieveDimMetrics(ctx context.Context, in *wrapperspb.BoolValue, opts ...grpc.CallOption) (*RetrieveDimMetricsResponse, error) {
	out := new(RetrieveDimMetricsResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.hps.HpsService/RetrieveDimMetrics", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hpsServiceClient) RetrieveHpsSenseSignal(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*HpsSenseSignalResponse, error) {
	out := new(HpsSenseSignalResponse)
	err := c.cc.Invoke(ctx, "/tast.cros.hps.HpsService/RetrieveHpsSenseSignal", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *hpsServiceClient) CheckSPAEyeIcon(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*wrapperspb.BoolValue, error) {
	out := new(wrapperspb.BoolValue)
	err := c.cc.Invoke(ctx, "/tast.cros.hps.HpsService/CheckSPAEyeIcon", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// HpsServiceServer is the server API for HpsService service.
type HpsServiceServer interface {
	// StartUIWithCustomScreenPrivacySetting changes the settings in screen privacy accordingly.
	StartUIWithCustomScreenPrivacySetting(context.Context, *StartUIWithCustomScreenPrivacySettingRequest) (*emptypb.Empty, error)
	// WaitForHps waits for hpsd to be ready, and optionally to finish enabling the requested features.
	// This includes booting the HPS peripheral and potentially flashing its firmware.
	WaitForHps(context.Context, *WaitForHpsRequest) (*emptypb.Empty, error)
	// CheckForLockScreen checks if the screen is at lock status.
	CheckForLockScreen(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	// OpenHPSInternalPage opens hps-internal page for debugging purpose.
	OpenHPSInternalsPage(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	// DimMetrics gets the quick dim/lock delays after the lol is enabled/disabled.
	RetrieveDimMetrics(context.Context, *wrapperspb.BoolValue) (*RetrieveDimMetricsResponse, error)
	// RetrieveHpsSenseSignal gets current HpsSenseSignal from powerd.
	RetrieveHpsSenseSignal(context.Context, *emptypb.Empty) (*HpsSenseSignalResponse, error)
	// CheckSPAEyeIcon checks if the eye icon is at the right bottom side of the screen when there is spa alert.
	CheckSPAEyeIcon(context.Context, *emptypb.Empty) (*wrapperspb.BoolValue, error)
}

// UnimplementedHpsServiceServer can be embedded to have forward compatible implementations.
type UnimplementedHpsServiceServer struct {
}

func (*UnimplementedHpsServiceServer) StartUIWithCustomScreenPrivacySetting(context.Context, *StartUIWithCustomScreenPrivacySettingRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StartUIWithCustomScreenPrivacySetting not implemented")
}
func (*UnimplementedHpsServiceServer) WaitForHps(context.Context, *WaitForHpsRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method WaitForHps not implemented")
}
func (*UnimplementedHpsServiceServer) CheckForLockScreen(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckForLockScreen not implemented")
}
func (*UnimplementedHpsServiceServer) OpenHPSInternalsPage(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method OpenHPSInternalsPage not implemented")
}
func (*UnimplementedHpsServiceServer) RetrieveDimMetrics(context.Context, *wrapperspb.BoolValue) (*RetrieveDimMetricsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RetrieveDimMetrics not implemented")
}
func (*UnimplementedHpsServiceServer) RetrieveHpsSenseSignal(context.Context, *emptypb.Empty) (*HpsSenseSignalResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RetrieveHpsSenseSignal not implemented")
}
func (*UnimplementedHpsServiceServer) CheckSPAEyeIcon(context.Context, *emptypb.Empty) (*wrapperspb.BoolValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckSPAEyeIcon not implemented")
}

func RegisterHpsServiceServer(s *grpc.Server, srv HpsServiceServer) {
	s.RegisterService(&_HpsService_serviceDesc, srv)
}

func _HpsService_StartUIWithCustomScreenPrivacySetting_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StartUIWithCustomScreenPrivacySettingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HpsServiceServer).StartUIWithCustomScreenPrivacySetting(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.hps.HpsService/StartUIWithCustomScreenPrivacySetting",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HpsServiceServer).StartUIWithCustomScreenPrivacySetting(ctx, req.(*StartUIWithCustomScreenPrivacySettingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _HpsService_WaitForHps_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(WaitForHpsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HpsServiceServer).WaitForHps(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.hps.HpsService/WaitForHps",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HpsServiceServer).WaitForHps(ctx, req.(*WaitForHpsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _HpsService_CheckForLockScreen_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HpsServiceServer).CheckForLockScreen(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.hps.HpsService/CheckForLockScreen",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HpsServiceServer).CheckForLockScreen(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _HpsService_OpenHPSInternalsPage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HpsServiceServer).OpenHPSInternalsPage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.hps.HpsService/OpenHPSInternalsPage",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HpsServiceServer).OpenHPSInternalsPage(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _HpsService_RetrieveDimMetrics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(wrapperspb.BoolValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HpsServiceServer).RetrieveDimMetrics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.hps.HpsService/RetrieveDimMetrics",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HpsServiceServer).RetrieveDimMetrics(ctx, req.(*wrapperspb.BoolValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _HpsService_RetrieveHpsSenseSignal_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HpsServiceServer).RetrieveHpsSenseSignal(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.hps.HpsService/RetrieveHpsSenseSignal",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HpsServiceServer).RetrieveHpsSenseSignal(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _HpsService_CheckSPAEyeIcon_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(HpsServiceServer).CheckSPAEyeIcon(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/tast.cros.hps.HpsService/CheckSPAEyeIcon",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(HpsServiceServer).CheckSPAEyeIcon(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _HpsService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "tast.cros.hps.HpsService",
	HandlerType: (*HpsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "StartUIWithCustomScreenPrivacySetting",
			Handler:    _HpsService_StartUIWithCustomScreenPrivacySetting_Handler,
		},
		{
			MethodName: "WaitForHps",
			Handler:    _HpsService_WaitForHps_Handler,
		},
		{
			MethodName: "CheckForLockScreen",
			Handler:    _HpsService_CheckForLockScreen_Handler,
		},
		{
			MethodName: "OpenHPSInternalsPage",
			Handler:    _HpsService_OpenHPSInternalsPage_Handler,
		},
		{
			MethodName: "RetrieveDimMetrics",
			Handler:    _HpsService_RetrieveDimMetrics_Handler,
		},
		{
			MethodName: "RetrieveHpsSenseSignal",
			Handler:    _HpsService_RetrieveHpsSenseSignal_Handler,
		},
		{
			MethodName: "CheckSPAEyeIcon",
			Handler:    _HpsService_CheckSPAEyeIcon_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "hps_setting_service.proto",
}

// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"context"
	"time"

	"chromiumos/tast/remote/bundles/cros/chameleond/util"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckChameleondServiceMethodsVideo,
		Desc: "Calls every available video gRPC endpoint in the ChameleondService as defined in the test cases",
		Contacts: []string{
			"jaredbennett@google.com",
		},
		Attr:         []string{},
		ServiceDeps:  []string{},
		SoftwareDeps: []string{},
		Fixture:      "simpleChameleond",
		Timeout:      1 * time.Minute,
		Params:       []testing.Param{
			// TODO rpc TriggerLinkFailure(TriggerLinkFailureRequest) returns (TriggerLinkFailureResponse);
			// TODO rpc HasVideoSupport(HasVideoSupportRequest) returns (HasVideoSupportResponse);
			// TODO rpc SetVgaMode(SetVgaModeRequest) returns (SetVgaModeResponse);
			// TODO rpc WaitVideoInputStable(WaitVideoInputStableRequest) returns (WaitVideoInputStableResponse);
			// TODO rpc CreateEdid(CreateEdidRequest) returns (CreateEdidResponse);
			// TODO rpc DestroyEdid(DestroyEdidRequest) returns (DestroyEdidResponse);
			// TODO rpc SetDdcState(SetDdcStateRequest) returns (SetDdcStateResponse);
			// TODO rpc IsDdcEnabled(IsDdcEnabledRequest) returns (IsDdcEnabledResponse);
			// TODO rpc ReadEdid(ReadEdidRequest) returns (ReadEdidResponse);
			// TODO rpc ApplyEdid(ApplyEdidRequest) returns (ApplyEdidResponse);
			// TODO rpc UnplugHPD(UnplugHPDRequest) returns (UnplugHPDResponse);
			// TODO rpc FireHpdPulse(FireHpdPulseRequest) returns (FireHpdPulseResponse);
			// TODO rpc FireMixedHpdPulses(FireMixedHpdPulsesRequest) returns (FireMixedHpdPulsesResponse);
			// TODO rpc ScheduleHpdToggle(ScheduleHpdToggleRequest) returns (ScheduleHpdToggleResponse);
			// TODO rpc SetContentProtection(SetContentProtectionRequest) returns (SetContentProtectionResponse);
			// TODO rpc IsContentProtectionEnabled(IsContentProtectionEnabledRequest) returns (IsContentProtectionEnabledResponse);
			// TODO rpc IsVideoInputEncrypted(IsVideoInputEncryptedRequest) returns (IsVideoInputEncryptedResponse);
			// TODO rpc DumpPixels(DumpPixelsRequest) returns (DumpPixelsResponse);
			// TODO rpc GetMaxFrameLimit(GetMaxFrameLimitRequest) returns (GetMaxFrameLimitResponse);
			// TODO rpc StartCapturingVideo(StartCapturingVideoRequest) returns (StartCapturingVideoResponse);
			// TODO rpc StopCapturingVideo(StopCapturingVideoRequest) returns (StopCapturingVideoResponse);
			// TODO rpc CaptureVideo(CaptureVideoRequest) returns (CaptureVideoResponse);
			// TODO rpc GetCapturedFrameCount(GetCapturedFrameCountRequest) returns (GetCapturedFrameCountResponse);
			// TODO rpc GetCapturedResolution(GetCapturedResolutionRequest) returns (GetCapturedResolutionResponse);
			// TODO rpc ReadCapturedFrame(ReadCapturedFrameRequest) returns (ReadCapturedFrameResponse);
			// TODO rpc CacheFrameThumbnail(CacheFrameThumbnailRequest) returns (CacheFrameThumbnailResponse);
			// TODO rpc GetCapturedChecksums(GetCapturedChecksumsRequest) returns (GetCapturedChecksumsResponse);
			// TODO rpc GetCapturedHistograms(GetCapturedHistogramsRequest) returns (GetCapturedHistogramsResponse);
			// TODO rpc ComputePixelChecksum(ComputePixelChecksumRequest) returns (ComputePixelChecksumResponse);
			// TODO rpc DetectResolution(DetectResolutionRequest) returns (DetectResolutionResponse);
			// TODO rpc GetVideoParams(GetVideoParamsRequest) returns (GetVideoParamsResponse);
			// TODO rpc GetLastInfoFrame(GetLastInfoFrameRequest) returns (GetLastInfoFrameResponse);
		},
	})
}

// CheckChameleondServiceMethodsVideo tests every video
// ChameleondService gRPC endpoint as defined in the test cases.
func CheckChameleondServiceMethodsVideo(ctx context.Context, s *testing.State) {
	util.CheckChameleondServiceMethods(ctx, s)
}

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
		Func: CheckChameleondServiceMethodsAudio,
		Desc: "Calls every available audio gRPC endpoint in the ChameleondService as defined in the test cases",
		Contacts: []string{
			"jaredbennett@google.com",
		},
		Attr:         []string{},
		ServiceDeps:  []string{},
		SoftwareDeps: []string{},
		Fixture:      "simpleChameleond",
		Timeout:      1 * time.Minute,
		Params:       []testing.Param{
			// TODO rpc HasAudioSupport(HasAudioSupportRequest) returns (HasAudioSupportResponse);
			// TODO rpc GetAudioChannelMapping(GetAudioChannelMappingRequest) returns (GetAudioChannelMappingResponse);
			// TODO rpc GetAudioFormat(GetAudioFormatRequest) returns (GetAudioFormatResponse);
			// TODO rpc StartCapturingAudio(StartCapturingAudioRequest) returns (StartCapturingAudioResponse);
			// TODO rpc StopCapturingAudio(StopCapturingAudioRequest) returns (StopCapturingAudioResponse);
			// TODO rpc StartPlayingAudio(StartPlayingAudioRequest) returns (StartPlayingAudioResponse);
			// TODO rpc StartPlayingEcho(StartPlayingEchoRequest) returns (StartPlayingEchoResponse);
			// TODO rpc StopPlayingAudio(StopPlayingAudioRequest) returns (StopPlayingAudioResponse);
			// TODO rpc AudioBoardConnect(AudioBoardConnectRequest) returns (AudioBoardConnectResponse);
			// TODO rpc AudioBoardDisconnect(AudioBoardDisconnectRequest) returns (AudioBoardDisconnectResponse);
			// TODO rpc AudioBoardGetRoutes(AudioBoardGetRoutesRequest) returns (AudioBoardGetRoutesResponse);
			// TODO rpc AudioBoardClearRoutes(AudioBoardClearRoutesRequest) returns (AudioBoardClearRoutesResponse);
			// TODO rpc AudioBoardHasJackPlugger(AudioBoardHasJackPluggerRequest) returns (AudioBoardHasJackPluggerResponse);
			// TODO rpc AudioBoardAudioJackPlug(AudioBoardAudioJackPlugRequest) returns (AudioBoardAudioJackPlugResponse);
			// TODO rpc AudioBoardAudioJackUnplug(AudioBoardAudioJackUnplugRequest) returns (AudioBoardAudioJackUnplugResponse);
			// TODO rpc SetUSBDriverPlaybackConfigs(SetUSBDriverPlaybackConfigsRequest) returns (SetUSBDriverPlaybackConfigsResponse);
			// TODO rpc SetUSBDriverCaptureConfigs(SetUSBDriverCaptureConfigsRequest) returns (SetUSBDriverCaptureConfigsResponse);
		},
	})
}

// CheckChameleondServiceMethodsAudio tests every audio
// ChameleondService gRPC endpoint as defined in the test cases.
func CheckChameleondServiceMethodsAudio(ctx context.Context, s *testing.State) {
	util.CheckChameleondServiceMethods(ctx, s)
}

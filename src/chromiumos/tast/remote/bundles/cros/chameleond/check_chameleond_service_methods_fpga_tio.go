// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chameleond

import (
	"context"
	"time"

	pbchameleond "go.chromium.org/chromiumos/config/go/platform/chameleon/chameleond/rpc"

	"chromiumos/tast/remote/bundles/cros/chameleond/util"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CheckChameleondServiceMethodsFpgaTio,
		Desc: "Calls every available fpga_tio gRPC endpoint in the ChameleondService as defined in the test cases",
		Contacts: []string{
			"jaredbennett@google.com",
		},
		Attr:         []string{},
		ServiceDeps:  []string{},
		SoftwareDeps: []string{},
		Fixture:      "simpleChameleond",
		Timeout:      1 * time.Minute,
		Params: []testing.Param{
			{
				Name: "start_monitoring_audio_video_capturing_delay",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.StartMonitoringAudioVideoCapturingDelay(ctx, &pbchameleond.StartMonitoringAudioVideoCapturingDelayRequest{})
					},
					ExpectedResponse: &pbchameleond.StartMonitoringAudioVideoCapturingDelayRequest{},
				},
			},
			{
				Name: "get_audio_video_capturing_delay",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.GetAudioVideoCapturingDelay(ctx, &pbchameleond.GetAudioVideoCapturingDelayRequest{})
					},
					ExpectedResponse: &pbchameleond.GetAudioVideoCapturingDelayResponse{},
				},
			},
			{
				Name: "has_audio_board",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.HasAudioBoard(ctx, &pbchameleond.HasAudioBoardRequest{})
					},
					ExpectedResponse: &pbchameleond.HasAudioBoardResponse{},
				},
			},
			{
				Name: "send_hid_event",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.SendHIDEvent(ctx, &pbchameleond.SendHIDEventRequest{
							PortId:    pbchameleond.PortId_HDMI,
							EventType: "test",
							EventArgs: []*pbchameleond.AnyScalar{
								{
									Scalar: &pbchameleond.AnyScalar_Int{Int: 123},
								},
							},
						})
					},
					ExpectedResponse: &pbchameleond.SendHIDEventResponse{},
				},
			},
			{
				Name: "enable_bluetooth_ref",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.EnableBluetoothRef(ctx, &pbchameleond.EnableBluetoothRefRequest{})
					},
					ExpectedResponse: &pbchameleond.EnableBluetoothRefResponse{},
				},
			},
			// This one reboots the chameleond device, so do it last.
			{
				Name: "reboot",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.Reboot(ctx, &pbchameleond.RebootRequest{})
					},
					ExpectedResponse: &pbchameleond.RebootResponse{},
				},
			},
		},
	})
}

// CheckChameleondServiceMethodsFpgaTio tests every fpga_tio
// ChameleondService gRPC endpoint as defined in the test cases.
func CheckChameleondServiceMethodsFpgaTio(ctx context.Context, s *testing.State) {
	util.CheckChameleondServiceMethods(ctx, s)
}

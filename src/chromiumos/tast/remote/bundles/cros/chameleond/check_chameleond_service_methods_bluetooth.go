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
		Func: CheckChameleondServiceMethodsBluetooth,
		Desc: "Calls every available bluetooth gRPC endpoint in the ChameleondService as defined in the test cases",
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
				Name: "audio_board_reset_bluetooth",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.AudioBoardResetBluetooth(ctx, &pbchameleond.AudioBoardResetBluetoothRequest{})
					},
					ExpectedResponse: &pbchameleond.AudioBoardResetBluetoothResponse{},
				},
			},
			{
				Name: "audio_board_disable_bluetooth",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.AudioBoardDisableBluetooth(ctx, &pbchameleond.AudioBoardDisableBluetoothRequest{})
					},
					ExpectedResponse: &pbchameleond.AudioBoardDisableBluetoothResponse{},
				},
			},
			{
				Name: "audio_board_is_bluetooth_enabled",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.AudioBoardIsBluetoothEnabled(ctx, &pbchameleond.AudioBoardIsBluetoothEnabledRequest{})
					},
					ExpectedResponse: &pbchameleond.AudioBoardIsBluetoothEnabledResponse{},
				},
			},
			{
				Name: "reset_bluetooth_ref",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.ResetBluetoothRef(ctx, &pbchameleond.ResetBluetoothRefRequest{})
					},
					ExpectedResponse: &pbchameleond.ResetBluetoothRefResponse{},
				},
			},
			{
				Name: "disable_bluetooth_ref",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.DisableBluetoothRef(ctx, &pbchameleond.DisableBluetoothRefRequest{})
					},
					ExpectedResponse: &pbchameleond.DisableBluetoothRefResponse{},
				},
			},
			{
				Name: "is_bluetooth_ref_disabled",
				Val: &util.CheckChameleondServiceMethodsTestCase{
					CallMethod: func(ctx context.Context, client pbchameleond.ChameleondServiceClient) (interface{}, error) {
						return client.IsBluetoothRefDisabled(ctx, &pbchameleond.IsBluetoothRefDisabledRequest{})
					},
					ExpectedResponse: &pbchameleond.IsBluetoothRefDisabledResponse{},
				},
			},
		},
	})
}

// CheckChameleondServiceMethodsBluetooth tests every bluetooth
// ChameleondService gRPC endpoint as defined in the test cases.
func CheckChameleondServiceMethodsBluetooth(ctx context.Context, s *testing.State) {
	util.CheckChameleondServiceMethods(ctx, s)
}

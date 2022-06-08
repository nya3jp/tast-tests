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
		Params:       []testing.Param{
			// TODO rpc AudioBoardResetBluetooth(AudioBoardResetBluetoothRequest) returns (AudioBoardResetBluetoothResponse);
			// TODO rpc AudioBoardDisableBluetooth(AudioBoardDisableBluetoothRequest) returns (AudioBoardDisableBluetoothResponse);
			// TODO rpc AudioBoardIsBluetoothEnabled(AudioBoardIsBluetoothEnabledRequest) returns (AudioBoardIsBluetoothEnabledResponse);
			// TODO rpc ResetBluetoothRef(ResetBluetoothRefRequest) returns (ResetBluetoothRefResponse);
			// TODO rpc DisableBluetoothRef(DisableBluetoothRefRequest) returns (DisableBluetoothRefResponse);
			// TODO rpc IsBluetoothRefDisabled(IsBluetoothRefDisabledRequest) returns (IsBluetoothRefDisabledResponse);
		},
	})
}

// CheckChameleondServiceMethodsBluetooth tests every bluetooth
// ChameleondService gRPC endpoint as defined in the test cases.
func CheckChameleondServiceMethodsBluetooth(ctx context.Context, s *testing.State) {
	util.CheckChameleondServiceMethods(ctx, s)
}

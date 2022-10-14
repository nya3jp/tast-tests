// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/remote/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestCanDiscover,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that btpeers can be set to emulate a type device and that the DUT can discover them as those devices",
		Contacts: []string{
			"jaredbennett@google.com",
			"cros-connectivity@google.com",
		},
		// TODO(b/245584709): Need to make new btpeer test attributes.
		Attr:         []string{},
		SoftwareDeps: []string{"chrome"},
		ServiceDeps:  []string{"tast.cros.bluetooth.BluetoothUIService"},
		Fixture:      "chromeLoggedInWithBluetoothEnabled",
		Timeout:      time.Second * 30,
	})
}

func TestCanDiscover(ctx context.Context, s *testing.State) {
	fv := s.FixtValue().(*bluetooth.FixtValue)

	if _, err := fv.BluetoothUIService.StartDiscovery(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to start discovery: ", err)
	}

	testing.Sleep(ctx, 3*time.Second)

	res, err := fv.BluetoothUIService.Devices(ctx, &emptypb.Empty{})
	if err != nil {
		s.Fatal("Failed to get devices: ", err)
	}

	for _, device := range res.DeviceInfos {
		testing.ContextLog(ctx, device.Name)
	}

	if _, err := fv.BluetoothUIService.StopDiscovery(ctx, &emptypb.Empty{}); err != nil {
		s.Fatal("Failed to stop discovery: ", err)
	}
}

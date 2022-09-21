// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BTPairMonoheadsetManual,
		Desc:         "Verify bluetooth mono-headset pair",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"bluetooth.monoHeadset"},
		Fixture:      "chromeLoggedIn",
		LacrosStatus: testing.LacrosVariantUnneeded,
	})
}

// BTPairMonoheadsetManual enables bluetooth and pair BT mono headset.
func BTPairMonoheadsetManual(ctx context.Context, s *testing.State) {
	monoHeadset := s.RequiredVar("bluetooth.monoHeadset")

	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		s.Fatal("Failed to get bluetooth adapters: ", err)
	}
	adapter := adapters[0]

	// Turn on bluetooth adapter.
	isPowered, err := adapter.Powered(ctx)
	if err != nil {
		s.Fatal("Failed to get powered property value: ", err)
	}
	if !isPowered {
		if err := adapter.SetPowered(ctx, true); err != nil {
			s.Fatal("Failed to turn on bluetooth adapter: ", err)
		}
	}

	if err := adapter.StartDiscovery(ctx); err != nil {
		s.Fatal("Failed to enable discovery: ", err)
	}

	var btDevice *bluez.Device
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		btDevice, err = bluez.DeviceByAlias(ctx, monoHeadset)
		if err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Timeout waiting for BT Mono Headset: ", err)
	}

	isPaired, err := btDevice.Paired(ctx)
	if !isPaired {
		if err := btDevice.Pair(ctx); err != nil {
			s.Fatal("Failed to pair bluetooth device: ", err)
		}
	}

	// Disconnect BT device.
	defer btDevice.Disconnect(ctx)

	// Connect BT device.
	isConnected, err := btDevice.Connected(ctx)
	if err != nil {
		s.Fatal("Failed to get BT connected status: ", err)
	}
	if !isConnected {
		if err := btDevice.Connect(ctx); err != nil {
			s.Fatal("Failed to connect bluetooth device: ", err)
		}
	}
}

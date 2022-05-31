// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network/netconfig"
	nc "chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/testing"
)

var nwProperties = nc.ConfigProperties{
	TypeConfig: nc.NetworkTypeConfigProperties{
		Wifi: &nc.WiFiConfigProperties{
			Ssid:       "testOpenWifi",
			Security:   nc.None,
			HiddenSsid: nc.Automatic}}}

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestConfigDuringOobe,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Example test for checking networks during OOBE",
		Contacts:     []string{"crisguerrero@chromium.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{}, // Manual execution only.
		Timeout:      1 * time.Minute,
	})
}

// TestConfigDuringOobe tests networks during OOBE using the network config API.
func TestConfigDuringOobe(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.DeferLogin())
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Set up connections")
	api, err := netconfig.NewCrosNetworkConfigOobe(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}
	defer api.Close(ctx)

	s.Log("(test) Making call to the network config api")
	p, err := api.GetDeviceStateList(ctx)
	if err != nil {
		s.Fatal("Failed to GetDeviceStateList: ", err)
	}
	defer api.Close(ctx)

	// guid, err := api.ConfigureNetwork(ctx, nwProperties, true)
	// if err != nil {
	// 	s.Fatal("Failed to configure network", err)
	// }
	// defer api.ForgetNetwork(ctx, guid)

	s.Logf("Properties: %s", p)
	s.Log("bye")
}

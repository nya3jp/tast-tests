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

func init() {
	testing.AddTest(&testing.Test{
		Func:         TestConfigDuringOobe,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Example test for checking network configuration during OOBE using the CrosNetworkConfig API",
		Contacts: []string{
			"crisguerrero@chromium.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
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

	api, err := netconfig.NewCrosNetworkConfigOobe(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}
	defer api.Close(ctx)

	var nwProperties = nc.ConfigProperties{
		TypeConfig: nc.NetworkTypeConfigProperties{
			Wifi: &nc.WiFiConfigProperties{
				Ssid:       "basicWifi",
				Security:   nc.None,
				HiddenSsid: nc.Automatic}}}

	guid, err := api.ConfigureNetwork(ctx, nwProperties, true)
	if err != nil {
		s.Fatal("Failed to configure network: ", err)
	}
	defer api.ForgetNetwork(ctx, guid)

	managedProperties, err := api.GetManagedProperties(ctx, guid)
	if err != nil {
		s.Fatalf("Failed to get managed properties for guid %s: %v", guid, err)
	}

	if managedProperties.TypeProperties.Wifi.Security !=
		nwProperties.TypeConfig.Wifi.Security ||
		managedProperties.TypeProperties.Wifi.Ssid.ActiveValue !=
			nwProperties.TypeConfig.Wifi.Ssid {
		s.Error("Set and expected config of test network do not match")
		s.Logf("Wifi set: %+v", managedProperties.TypeProperties.Wifi)
		s.Logf("Wifi expected: %+v", nwProperties.TypeConfig.Wifi)
	}
}

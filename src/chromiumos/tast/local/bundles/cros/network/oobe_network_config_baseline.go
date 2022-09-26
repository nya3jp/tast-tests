// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network/netconfig"
	nc "chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         OobeNetworkConfigBaseline,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Use the CrosNetworkConfig API during OOBE to configure a basic network configuration and check it is setup as expected",
		Contacts: []string{
			"crisguerrero@chromium.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      1 * time.Minute,
	})
}

// OobeNetworkConfigBaseline tests a basic network configuration during OOBE
// using the CrosNetworkConfig API. This test is intended as a baseline example
// of how to use such API during OOBE.
func OobeNetworkConfigBaseline(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.DeferLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	api, err := netconfig.CreateOobeCrosNetworkConfig(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}
	defer api.Close(cleanupCtx)

	var configProperties = nc.ConfigProperties{
		TypeConfig: nc.NetworkTypeConfigProperties{
			Wifi: &nc.WiFiConfigProperties{
				Ssid:       "basicWifi",
				Security:   nc.None,
				HiddenSsid: nc.Automatic}}}

	guid, err := api.ConfigureNetwork(ctx, configProperties, true)
	if err != nil {
		s.Fatal("Failed to configure network: ", err)
	}
	defer func(ctx context.Context) {
		if success, err := api.ForgetNetwork(ctx, guid); err != nil {
			s.Fatalf("Failed to forget network with guid %s: %v", guid, err)
		} else if success != true {
			s.Fatalf("Request to forget network with guid %s was not successful", guid)
		}
	}(cleanupCtx)

	managedProperties, err := api.GetManagedProperties(ctx, guid)
	if err != nil {
		s.Fatalf("Failed to get managed properties for guid %s: %v", guid, err)
	}

	if managedProperties.TypeProperties.Wifi.Security !=
		configProperties.TypeConfig.Wifi.Security ||
		managedProperties.TypeProperties.Wifi.Ssid.ActiveValue !=
			configProperties.TypeConfig.Wifi.Ssid {
		s.Errorf("Set and expected config of test network do not match. Got: %+v. Want: %+v", managedProperties.TypeProperties.Wifi, configProperties.TypeConfig.Wifi)
	}
}

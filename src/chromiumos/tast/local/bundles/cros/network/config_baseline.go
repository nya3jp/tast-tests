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

type testParam struct {
	useLogin bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ConfigBaseline,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Use the CrosNetworkConfig API during OOBE and after login to configure a basic network configuration and check it is setup as expected",
		Contacts: []string{
			"crisguerrero@chromium.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      1 * time.Minute,
		Params: []testing.Param{{
			Name: "oobe",
			Val: testParam{
				useLogin: false,
			},
		}, {
			Name: "login",
			Val: testParam{
				useLogin: true,
			},
		},
		},
	})
}

// ConfigBaseline tests a basic network configuration during OOBE or after login
// using the CrosNetworkConfig API. This test is intended as a baseline example
// of how to use such API and make sure it works.
// Various more complex tests rely on the network API to work, so it is
// important it runs in the CQ to detect possible issues with it on time.
func ConfigBaseline(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	param := s.Param().(testParam)
	var opts []chrome.Option
	if !param.useLogin {
		// Defer login to use the API during OOBE.
		opts = []chrome.Option{
			chrome.DeferLogin(),
		}
	}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	// How we connect to the API depends on if we are in the OOBE or have access
	// to a chrome instance after login.
	var api *nc.CrosNetworkConfig
	if param.useLogin {
		// Connect using chrome.
		api, err = netconfig.CreateLoggedInCrosNetworkConfig(ctx, cr)
	} else {
		// Connect using OOBE.
		api, err = netconfig.CreateOobeCrosNetworkConfig(ctx, cr)
	}
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

// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MojoCellularSuspendResumeAutoconnect, LacrosStatus: testing.LacrosVariantUnknown, Desc: "Verifies using Mojo API that cellular maintains autoconnect state around Suspend/Resume",
		Contacts: []string{
			"danielwinkler@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:         []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture:      "cellular",
		Timeout:      2 * time.Minute,
		LacrosStatus: testing.LacrosVariantUnNeeded,
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name: "autoconnect_enabled",
			Val:  true,
		}, {
			Name: "autoconnect_disabled",
			Val:  false,
		}},
	})
}

// MojoCellularSuspendResumeAutoconnect sets autoconnect property to true/false using Mojo API suspends the DUT
// and verifies whether network is online/offline using Shill.
// Wifi and Ethernet is disabled before the test
func MojoCellularSuspendResumeAutoconnect(ctx context.Context, s *testing.State) {

	//cleanupCtx := ctx
	autoconnectState := s.Param().(bool)

	expectedStates := map[bool]string{true: shillconst.ServiceStateOnline,
		false: shillconst.ServiceStateIdle}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Disable Ethernet and/or WiFi if present and defer re-enabling.
	// Shill documentation shows that autoconnect will only be used if there
	// is no other service available, so it is necessary to only have
	// cellular available.

	if enableFunc, err := helper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyEthernet); err != nil {
		s.Fatal("Unable to disable Ethernet: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
		defer cancel()
		defer enableFunc(ctx)
		ctx = newCtx
	}

	if enableFunc, err := helper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyWifi); err != nil {
		s.Fatal("Unable to disable Wifi: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
		defer cancel()
		defer enableFunc(ctx)
		ctx = newCtx
	}

	// Enable and get service to set autoconnect based on test parameters.
	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}

	if isConnected, err := service.IsConnected(ctx); err != nil {
		s.Fatal("Unable to get IsConnected for Service: ", err)
	} else {
		if !isConnected {
			if _, err := helper.ConnectToDefault(ctx); err != nil {
				s.Fatal("Unable to Connect to Service: ", err)
			}
		}
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed start Chrome: ", err)
	}
	defer cr.Close(ctx)

	netConn, err := netconfig.NewCrosNetworkConfig(ctx, cr)
	if err != nil {
		s.Fatal("Failed to get network Mojo Object: ", err)
	}

	// Get state of all active cellular networks
	filter := netconfig.NetworkFilter{
		Filter:      netconfig.ActiveFT,
		NetworkType: netconfig.Cellular,
		Limit:       0}

	// Get the GUID for cellular network
	networkState, err := netConn.GetNetworkStateList(ctx, filter)
	if err != nil {
		s.Fatal("Failed to get NetworkStateList: ", err)
	}

	GUID := ""

	for _, cfg := range networkState {
		if cfg.Type == netconfig.Cellular {
			GUID = cfg.GUID
		}
	}
	if GUID == "" {
		s.Fatal("Unable to find GUID for cellular network")
	} else {
		s.Logf("GUID is %s", GUID)
	}

	cfgProperties := netconfig.ConfigProperties{
		AutoConnect: netconfig.AutoConnectConfig{Value: autoconnectState},
		GUID:        GUID,
		TypeConfig: netconfig.NetworkTypeConfigProperties{
			Cellular: netconfig.CellularConfigProperties{
				Apn:     netconfig.ApnProperties{AccessPointName: "foo"}, // Can this be removed
				Roaming: netconfig.RoamingProperties{AllowRoaming: false},
			}}}

	//if err = netConn.SetProperties(ctx, GUID, cfgProperties, "cellular"); err != nil {
	if err = netConn.SetProperties(ctx, GUID, cfgProperties, ""); err != nil {
		s.Fatal("Failed to set properties: ", err)
	}

	// Request suspend for 10 seconds.
	if err := testexec.CommandContext(ctx, "powerd_dbus_suspend", "--suspend_for_sec=10").Run(); err != nil {
		s.Fatal("Failed to perform system suspend: ", err)
	}

	// The reconnection will not occur from the login screen, so we log in.
	cr, err = chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}

	//defer upstart.RestartJob(ctx, "ui")

	service, err = helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}

	// Ensure service's state matches expectations.
	if err := service.WaitForProperty(ctx, shillconst.ServicePropertyState, expectedStates[autoconnectState], 60*time.Second); err != nil {
		s.Fatal("Failed to get service state: ", err)
	}

}

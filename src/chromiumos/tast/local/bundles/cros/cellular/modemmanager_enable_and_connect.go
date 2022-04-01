// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemmanagerEnableAndConnect,
		Desc:     "Verifies that modemmanager can trigger modem enable, disable, connect and disconnect succeeds",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Fixture:  "cellular",
		Timeout:  5 * time.Minute,
	})
}

// ModemmanagerEnableAndConnect Test
func ModemmanagerEnableAndConnect(ctx context.Context, s *testing.State) {
	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// Disable AutoConnect so that enable does not connect.
	ctxForAutoConnectCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cellular.AutoConnectCleanupTime)
	defer cancel()
	if wasAutoConnect, err := helper.SetServiceAutoConnect(ctx, false); err != nil {
		s.Fatal("Failed to disable AutoConnect: ", err)
	} else if wasAutoConnect {
		defer func(ctx context.Context) {
			if _, err := helper.SetServiceAutoConnect(ctx, true); err != nil {
				s.Fatal("Failed to enable AutoConnect: ", err)
			}
		}(ctxForAutoConnectCleanUp)
	}

	// Disabling cellular in shill, prevents shill from re-enabling cellular
	// after Modem disable called.
	if enableFunc, err := helper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyCellular); err != nil {
		s.Fatal("Unable to disable Cellular: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime+5*time.Second)
		defer cancel()
		defer enableFunc(ctx)
		// Restart ModemManager after test
		defer helper.RestartModemManager(ctx, true)
		ctx = newCtx
	}

	// Test Disable / Enable / Connect / Disconnect.
	s.Log("Disable")
	if err := modem.Call(ctx, mmconst.ModemEnable, false).Err; err != nil {
		s.Fatal("Modem disable failed with: ", err)
	}
	if err := modemmanager.EnsureDisabled(ctx, modem); err != nil {
		s.Fatal("Modem not disabled: ", err)
	}

	// Delay after disable is needed as certain qualcomm modems failed to
	// connect after registered state on some boards,
	// TODO: Remove sleep once b/188448918(duplicate: b/200644653) fixed.
	testing.Sleep(ctx, 2*time.Second)
	s.Log("Enable")
	if err := modem.Call(ctx, mmconst.ModemEnable, true).Err; err != nil {
		s.Fatal("Modem enable failed with: ", err)
	}

	if err := modemmanager.EnsureEnabled(ctx, modem); err != nil {
		s.Fatal("Modem not enabled: ", err)
	}
	s.Log("Modem disable-enable done")

	simpleModem, err := modem.GetSimpleModem(ctx)
	if err != nil {
		s.Fatal("Could not get simplemodem object: ", err)
	}

	// Check registration state and wait 60 seconds, modem scans for networks
	// and registers. TODO: b/199331589 push to register if any modem needs it.
	if err := modemmanager.EnsureRegistered(ctx, modem, simpleModem); err != nil {
		s.Fatal("Modem not registered: ", err)
	}

	s.Log("Connect")
	simpleConnectProps := map[string]interface{}{"apn": ""}

	// TODO(b/211015303): simplemodem connect with empty apn fails on few carriers.
	// Get apn from shill properties and try this apn if empty apn connect fails.
	apn := getApn(ctx, helper)

	// Connect and poll for modem state.
	if err := modemmanager.Connect(ctx, simpleModem, simpleConnectProps, 30*time.Second); err != nil {
		// Retry with first apn from Cellular.APNList.
		simpleConnectProps = map[string]interface{}{"apn": apn}
		s.Log("Empty apn connect failed with: ", err)
		if err = modemmanager.Connect(ctx, simpleModem, simpleConnectProps, 30*time.Second); err != nil {
			s.Fatal("Modem connect failed with error: ", err)
		}
	}

	if err := modemmanager.EnsureConnectState(ctx, modem, simpleModem, true); err != nil {
		s.Fatal("Modem not connected: ", err)
	}

	s.Log("Disconnect")
	if err := simpleModem.Call(ctx, mmconst.ModemDisconnect, dbus.ObjectPath("/")).Err; err != nil {
		s.Fatal("Modem disconnect failed with: ", err)
	}

	if err := modemmanager.EnsureConnectState(ctx, modem, simpleModem, false); err != nil {
		s.Fatal("Modem not disconnected: ", err)
	}
}

// getApn returns first available apn from Cellular.APNList.
func getApn(ctx context.Context, helper *cellular.Helper) string {
	props, _ := helper.Device.GetShillProperties(ctx)
	apns, err := props.Get(shillconst.DevicePropertyCellularAPNList)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get cellular device properties")
	}

	apnList, ok := apns.([]map[string]string)
	if !ok {
		testing.ContextLog(ctx, "Invalid format for cellular apn list")
	}

	for i := 0; i < len(apnList); i++ {
		apn := apnList[i]["apn"]
		if len(apn) > 0 {
			testing.ContextLog(ctx, "First apn in apn list: ", apn)
			return apn
		}
	}
	return ""
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ModemmanagerEnableAndConnect,
		Desc:     "Verifies that modemmanager can trigger modem enable, disable, connect and disconnect succeeds",
		Contacts: []string{"srikanthkumar@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "cellular_unstable"},
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
	if enableFunc, err := helper.Manager.DisableTechnologyForTesting(ctx, shill.TechnologyCellular); err != nil {
		s.Fatal("Unable to disable Cellular: ", err)
	} else if enableFunc != nil {
		newCtx, cancel := ctxutil.Shorten(ctx, shill.EnableWaitTime)
		defer cancel()
		defer enableFunc(ctx)
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

	s.Log("Enable")
	if err := modem.Call(ctx, mmconst.ModemEnable, true).Err; err != nil {
		s.Fatal("Modem enable failed with: ", err)
	}

	if err := modemmanager.EnsureEnabled(ctx, modem); err != nil {
		s.Fatal("Modem not enabled: ", err)
	}

	s.Log("Modem disable-enable done")

	apn, err := findAPN(ctx)
	if err != nil {
		s.Fatal("Failed to get apn: ", err)
	}
	simpleConnectProps := map[string]interface{}{"allow-roaming": true, "apn": apn}
	simpleModem, err := modem.GetSimpleModem(ctx)
	if err != nil {
		s.Fatal("Could not get simplemodem object: ", err)
	}

	s.Log("Connect")
	if err := simpleModem.Call(ctx, mmconst.ModemConnect, simpleConnectProps).Err; err != nil {
		s.Fatal("Modem connect failed with: ", err)
	}

	if err := modemmanager.EnsureConnectState(ctx, modem, simpleModem, true); err != nil {
		s.Fatal("Modem not connected: ", err)
	}

	s.Log("Disconnect")
	if err := simpleModem.Call(ctx, mmconst.ModemDisconnect, "/").Err; err != nil {
		s.Fatal("Modem disconnect failed with: ", err)
	}

	if err := modemmanager.EnsureConnectState(ctx, modem, simpleModem, false); err != nil {
		s.Fatal("Modem not disconnected: ", err)
	}
	s.Log("Test Done")
}

// findAPN finds last used good apn if any
func findAPN(ctx context.Context) (string, error) {
	emptyApn := "None"
	lastGoodApn := ""
	if lastGoodApn == "" {
		return emptyApn, nil
	}
	return lastGoodApn, nil
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
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

	simpleConnectProps := map[string]interface{}{"apn": ""}
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
	// Connect and poll for modem state.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		simpleModem.Call(ctx, mmconst.ModemConnect, simpleConnectProps)
		if isConnected, err := simpleModem.IsConnected(ctx); err != nil {
			return errors.Wrap(err, "failed to fetch connected state")
		} else if !isConnected {
			return errors.Wrap(err, "modem not connected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: 2 * time.Second,
	}); err != nil {
		if errmm := upstart.RestartJob(ctx, "modemmanager"); errmm != nil {
			s.Log("Failed to restart modemmanager: ", errmm)
		}
		s.Fatal("Modem connect failed with: ", err)
	}

	if err := modemmanager.EnsureConnectState(ctx, modem, simpleModem, true); err != nil {
		if errmm := upstart.RestartJob(ctx, "modemmanager"); errmm != nil {
			s.Log("Failed to restart modemmanager: ", errmm)
		}
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

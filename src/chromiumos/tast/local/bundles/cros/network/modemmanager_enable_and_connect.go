// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/modemmanager"
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
	props, err := modem.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call GetProperties on modem")
	}
	sim, err := props.GetObjectPath(mmconst.ModemPropertySim)
	if err != nil {
		s.Fatal("Missing Sim property: ", err)
	}
	s.Log("SIM path = ", sim)
	simSlots, err := props.GetObjectPaths(mmconst.ModemPropertySimSlots)
	if err != nil {
		s.Fatal("Failed to get SimSlots property: ", err)
	}
	if len(simSlots) == 0 {
		s.Log("No SimSlots for device, ending test")
		return
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

	// Test Disable / Enable / Connect / Disconnect.
	s.Log("Disable")
	disable(ctx, modem)

	s.Log("EnsureDisabled")
	ensureDisabled(ctx, modem)

	s.Log("Enable")
	enable(ctx, modem)

	s.Log("EnsureEnabled")
	ensureEnabled(ctx, modem)

	s.Log("Modem disable enable done")

	apn, err := findAPN(ctx)
	if err != nil {
		s.Fatal("Failed to get apn: ", err)
	}
	simpleConnectProps := map[string]string{"allow-roaming": "true", "apn": apn}
	// Connect - SimpleModem.Connect
	simpleModem, err := modem.GetSimpleModem(ctx)
	if err != nil {
		s.Fatal("Could not get simplemodem object: ", err)
	}

	s.Log("Connect")
	connect(ctx, simpleModem, simpleConnectProps)

	s.Log("EnsureConnected")
	ensureConnected(ctx, modem, simpleModem)

	s.Log("Disconnect")
	disconnect(ctx, simpleModem)

	s.Log("EnsureDisconnected")
	ensureDisconnected(ctx, modem, simpleModem)
	s.Log("Test Done")
}

// findAPN finds last used good apn if any
func findAPN(ctx context.Context) (string, error) {
	emptyApn := "None"
	lastGoodApn := ""
	// TODO: find cellular service and last good apn
	//service = self.test_env.shill.find_cellular_service_object()
	//lastGoodApn = self.test_env.shill.get_dbus_property(
	//		service,
	//		cellular_proxy.CellularProxy.SERVICE_PROPERTY_LAST_GOOD_APN)
	if lastGoodApn == "" {
		return emptyApn, nil
	}
	//return lastGoodApn.get(
	//		cellular_proxy.CellularProxy.APN_INFO_PROPERTY_APN, default)
	return lastGoodApn, nil
}

// enable enables modem
func enable(ctx context.Context, modem *modemmanager.Modem) error {
	// enable modem, call on modem object, TODO:handle already enabled case
	if err := modem.Call(ctx, "Enable", true).Err; err != nil {
		return errors.Errorf("Enable failed: ", err)
	}
	return nil
}

// ensureEnabled checks modem property enabled
func ensureEnabled(ctx context.Context, modem *modemmanager.Modem) error {
	isPowered, err := modem.IsPowered(ctx)
	if err != nil {
		return errors.New("failed to read modem powered state")
	}
	if !isPowered {
		return errors.New("modem not powered")
	}

	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isEnabled, err := modem.IsEnabled(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch enabled state: ", err)
		}
		if !isEnabled {
			return errors.New("modem not enabled")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Errorf("failed to enable modem: ", err)
	}
	return nil
}

// disable disables modem
func disable(ctx context.Context, modem *modemmanager.Modem) error {
	// Disable modem, call on modem object, TODO: handle already disabled case
	if err := modem.Call(ctx, "Disable", true).Err; err != nil {
		return errors.Errorf("Disable failed: ", err)
	}
	return nil
}

// ensureDisabled checks modem property disabled
func ensureDisabled(ctx context.Context, modem *modemmanager.Modem) error {
	// TODO:This check needs validated across modems
	isPowered, err := modem.IsPowered(ctx)
	if err != nil {
		return errors.Errorf("failed to read modem powered state: ", err)
	}
	if isPowered {
		return errors.New("modem still in powered state")
	}
	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isEnabled, err := modem.IsEnabled(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch enabled state: ", err)
		}
		isDisabled, err := modem.IsDisabled(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch disabled state: ", err)
		}
		if isEnabled || !isDisabled {
			return errors.New("still modem not disabled")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Errorf("failed to disable modem: ", err)
	}
	return nil
}

// connect connects modem and creates bearer
func connect(ctx context.Context, simpleModem *modemmanager.Modem, simpleProps map[string]string) error {
	// Connect modem, call on simplemodem object
	if err := simpleModem.Call(ctx, "Connect", simpleProps).Err; err != nil {
		return errors.Errorf("Connect failed: ", err)
	}
	return nil
}

// ensureConnected checks modem state property from simple modem GetStatus
func ensureConnected(ctx context.Context, modem, simpleModem *modemmanager.Modem) error {
	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ensureEnabled(ctx, modem)
		isConnected, err := simpleModem.IsConnected(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch connected state: ", err)
		}
		if !isConnected {
			return errors.New("still not connected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Errorf("failed to connect modem: ", err)
	}
	return nil
}

// disconnect disconnects simplemodem and remove bearer
func disconnect(ctx context.Context, simpleModem *modemmanager.Modem) error {
	// disconnect modem, call on simplemodem object
	if err := simpleModem.Call(ctx, "Disconnect", "/").Err; err != nil {
		return errors.Errorf("Disconnect failed: ", err)
	}
	return nil
}

// ensureDisconnected checks modem property disconnected
func ensureDisconnected(ctx context.Context, modem, simpleModem *modemmanager.Modem) error {
	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		ensureEnabled(ctx, modem)
		isConnected, err := simpleModem.IsConnected(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch connected state: ", err)
		}
		if isConnected {
			return errors.New("still not disconnected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Errorf("failed to disconnect modem: ", err)
	}
	return nil
}

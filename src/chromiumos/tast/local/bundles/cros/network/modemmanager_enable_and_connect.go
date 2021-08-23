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
	Disable(ctx, modem)

	s.Log("EnsureDisabled")
	EnsureDisabled(ctx, modem)

	s.Log("Enable")
	Enable(ctx, modem)

	s.Log("EnsureEnabled")
	EnsureEnabled(ctx, modem)

	s.Log("Modem disable enable done")

	apn, err := FindAPN(ctx)
	if err != nil {
		s.Fatal("Failed to get apn: ", err)
	}
	simple_connect_props := map[string]string{"allow-roaming": "true", "apn": apn}
	// Connect - SimpleModem.Connect
	simpleModem, err := modem.GetSimpleModem(ctx)
	if err != nil {
		s.Fatal("Could not get simplemodem object", err)
	}

	s.Log("Connect")
	Connect(ctx, simpleModem, simple_connect_props)

	s.Log("EnsureConnected")
	EnsureConnected(ctx, modem, simpleModem)

	s.Log("Disconnect")
	Disconnect(ctx, simpleModem)

	s.Log("EnsureDisconnected")
	EnsureDisconnected(ctx, modem, simpleModem)
	s.Log("Test Done")
}

// FindAPN finds last used good apn if any
func FindAPN(ctx context.Context) (string, error) {
	empty_apn := "None"
	last_good_apn := ""
	// TODO: find cellular service and last good apn
	//service = self.test_env.shill.find_cellular_service_object()
	//last_good_apn = self.test_env.shill.get_dbus_property(
	//		service,
	//		cellular_proxy.CellularProxy.SERVICE_PROPERTY_LAST_GOOD_APN)
	if last_good_apn == "" {
		return empty_apn, nil
	}
	//return last_good_apn.get(
	//		cellular_proxy.CellularProxy.APN_INFO_PROPERTY_APN, default)
	return last_good_apn, nil
}

// Enable enables modem
func Enable(ctx context.Context, modem *modemmanager.Modem) error {
	// Enable modem, call on modem object, handle already enabled case
	if err := modem.Call(ctx, "Enable", true).Err; err != nil {
		return errors.Errorf("Enable failed: ", err)
	}
	return nil
}

// EnsureEnabled checks modem property enabled
func EnsureEnabled(ctx context.Context, modem *modemmanager.Modem) error {
	isPowered, err := modem.IsPowered(ctx)
	if err != nil {
		return errors.Errorf("failed to read modem powered state")
	}
	if !isPowered {
		return errors.Errorf("modem not powered")
	}

	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isEnabled, err := modem.IsEnabled(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch enabled state", err)
		}
		if !isEnabled {
			return errors.Errorf("modem not enabled")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Errorf("failed to enable modem", err)
	}
	return nil
}

// Disable disables modem
func Disable(ctx context.Context, modem *modemmanager.Modem) error {
	// Disable modem, call on modem object, TODO: handle already disabled case
	if err := modem.Call(ctx, "Disable", true).Err; err != nil {
		return errors.Errorf("Disable failed: ", err)
	}
	return nil
}

// EnsureDisabled checks modem property disabled
func EnsureDisabled(ctx context.Context, modem *modemmanager.Modem) error {
	// TODO:This check needs validated across modems
	isPowered, err := modem.IsPowered(ctx)
	if err != nil {
		return errors.Errorf("failed to read modem powered state")
	}
	if isPowered {
		return errors.Errorf("modem still in powered state")
	}
	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		isEnabled, err := modem.IsEnabled(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch enabled state", err)
		}
		isDisabled, err := modem.IsDisabled(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch disabled state", err)
		}
		if isEnabled || !isDisabled {
			return errors.Errorf("still not disabled")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Errorf("failed to disable modem", err)
	}
	return nil
}

// Connect connects modem and creates bearer
func Connect(ctx context.Context, simpleModem *modemmanager.Modem, simple_props map[string]string) error {
	// Connect modem, call on simplemodem object
	if err := simpleModem.Call(ctx, "Connect", simple_props).Err; err != nil {
		return errors.Errorf("Connect failed: ", err)
	}
	return nil
}

// EnsureConnected checks modem state property from simple modem GetStatus
func EnsureConnected(ctx context.Context, modem *modemmanager.Modem, simpleModem *modemmanager.Modem) error {
	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		EnsureEnabled(ctx, modem)
		isConnected, err := simpleModem.IsConnected(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch connected state", err)
		}
		if !isConnected {
			return errors.Errorf("still not connected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Errorf("failed to connect modem", err)
	}
	return nil
}

// Disconnect disconnects simplemodem and remove bearer
func Disconnect(ctx context.Context, simpleModem *modemmanager.Modem) error {
	// Disconnect modem, call on simplemodem object
	if err := simpleModem.Call(ctx, "Disconnect", "/").Err; err != nil {
		return errors.Errorf("Disconnect failed: ", err)
	}
	return nil
}

// EnsureDisconnected checks modem property disconnected
func EnsureDisconnected(ctx context.Context, modem *modemmanager.Modem, simpleModem *modemmanager.Modem) error {
	// poll for expected modem state
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		EnsureEnabled(ctx, modem)
		isConnected, err := simpleModem.IsConnected(ctx)
		if err != nil {
			return errors.Errorf("failed to fetch connected state", err)
		}
		if isConnected {
			return errors.Errorf("still not disconnected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		return errors.Errorf("failed to disconnect modem", err)
	}
	return nil
}

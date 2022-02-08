// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

type testParam2 struct {
	ModbOverrideProto     string
	ExpectedLastAttachAPN string
	ExpectedLastGoodAPN   string
	// Configure an Attach APN before starting the test.
	SetInitialAttachAPNValue map[string]interface{}
	// Connect to APN
	ApnToConnect map[string]interface{}
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularConnectToUnknownMno,
		Desc: "Verifies that traffic can be sent over the Cellular network",
		Contacts: []string{
			"andrewlassalle@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr: []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Params: []testing.Param{{
			Name:      "unknown_carrier",
			Val:       testParam2{"amari_unknown_carrier.pbf", "amari_default_attach", "amari_ipv4", map[string]interface{}{"apn": "wrong_attach"}, map[string]interface{}{"apn": "amari_ipv4"}},
			ExtraData: []string{"amari_unknown_carrier.pbf"},
		}},
		Fixture: "cellular",
		Timeout: 1 * time.Minute,
	})
}

func ShillCellularConnectToUnknownMno(ctx context.Context, s *testing.State) {
	modbOverrideProto := s.Param().(testParam2).ModbOverrideProto
	expectedLastGoodAPN := s.Param().(testParam2).ExpectedLastGoodAPN
	expectedLastAttachAPN := s.Param().(testParam2).ExpectedLastAttachAPN
	setInitialAttachAPNValue := s.Param().(testParam2).SetInitialAttachAPNValue
	apnToConnect := s.Param().(testParam2).ApnToConnect
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	modem3gpp, err := modem.GetModem3gpp(ctx)
	if err != nil {
		s.Fatal("Could not get modem3gpp object: ", err)
	}
	if setInitialAttachAPNValue != nil {
		if err := modemmanager.SetInitialEpsBearerSettings(ctx, modem3gpp, setInitialAttachAPNValue); err != nil {
			s.Fatal("Failed to set initial EPS bearer settings: ", err)
		}
	}

	if _, err = helper.Disable(ctx); err != nil {
		s.Fatal("Failed to disable cellular: ", err)
	}

	err = cellular.SetServiceProvidersOverride(ctx, s.DataPath(modbOverrideProto))
	if err != nil {
		s.Fatal("Failed to set service providers override: ", err)
	}
	defer os.Remove("/usr/share/shill/serviceproviders-override.pbf") // TODO: change with constant.
	errs := helper.ResetShill(ctx)                                    //TODO: change array to simple error?
	if errs != nil {
		s.Fatal("Failed to reset shill: ", err)
	}

	if _, err = helper.Enable(ctx); err != nil {
		s.Fatal("Failed to enable cellular: ", err)
	}

	// Verify that a connectable Cellular service exists and ensure it is connected.

	if _, err := helper.FindServiceForDevice(ctx); err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}

	if err := modem.WaitForState(ctx, mmconst.ModemStateRegistered, 20*time.Second); err != nil {
		s.Fatal("Modem is not registered")
	}

	simpleModem, err := modem.GetSimpleModem(ctx)
	if err != nil {
		s.Fatal("Could not get simplemodem object: ", err)
	}

	testing.ContextLog(ctx, "Connecting")
	if err := modemmanager.Connect(ctx, simpleModem, apnToConnect, 20*time.Second); err != nil {
		s.Fatal("Modem connect failed with error: ", err)

	}

	modemAttachApn, err := modemmanager.GetInitialEpsBearerSettings(ctx, modem)
	// modemAttachApn := initialEpsBearerSettings.(map[string]interface)
	if err != nil {
		s.Fatal("Error getting Service properties: ", err)
	}
	//TODO: Get the `connect` APN from MM, since shill will not have that info in LastGoodAPN
	serviceLastGoodAPN, err := helper.GetCellularLastGoodAPN(ctx)
	if err != nil {
		s.Fatal("Error getting Service properties: ", err)
	}
	testing.ContextLog(ctx, "modemAttachApn:", modemAttachApn)
	testing.ContextLog(ctx, "serviceAPN", serviceLastGoodAPN)

	apnName := modemAttachApn["apn"]
	if apnName != expectedLastAttachAPN {
		s.Fatalf("last Attach APN doesn't match. Current Attach is %q, expected is %q", apnName, expectedLastAttachAPN)
	}

	apnName = serviceLastGoodAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
	if apnName != expectedLastGoodAPN {
		s.Fatalf("last good APN doesn't match. Current Attach is %q, expected is %q", apnName, expectedLastGoodAPN)
	}

	// TODO: do some basic connectivity test. Check IP type.
}

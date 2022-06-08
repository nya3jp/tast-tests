// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

type unknownMNOTestParam struct {
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
		Attr: []string{"group:cellular", "cellular_sim_active", "cellular_unstable", "cellular_amari_callbox"},
		Params: []testing.Param{{
			Name:      "unknown_carrier",
			Val:       unknownMNOTestParam{"callbox_unknown_carrier.pbf", "callbox-default-attach", "callbox-ipv4", map[string]interface{}{"apn": "wrong_attach"}, map[string]interface{}{"apn": "callbox-ipv4"}},
			ExtraData: []string{"callbox_unknown_carrier.pbf"},
		}},
		Fixture: "cellular",
		Timeout: 1 * time.Minute,
	})
}

func ShillCellularConnectToUnknownMno(ctx context.Context, s *testing.State) {
	params := s.Param().(unknownMNOTestParam)
	modbOverrideProto := params.ModbOverrideProto
	expectedLastGoodAPN := params.ExpectedLastGoodAPN
	expectedLastAttachAPN := params.ExpectedLastAttachAPN
	setInitialAttachAPNValue := params.SetInitialAttachAPNValue
	apnToConnect := params.ApnToConnect
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

	deferCleanUp, err := cellular.SetServiceProvidersExclusiveOverride(ctx, s.DataPath(modbOverrideProto))
	if err != nil {
		s.Fatal("Failed to set service providers override: ", err)
	}
	defer deferCleanUp()
	errs := helper.ResetShill(ctx)
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

	modemAttachApn, err := modem.GetInitialEpsBearerSettings(ctx, modem)
	if err != nil {
		s.Fatal("Error getting Attach APN properties: ", err)
	}
	connectApn, err := modem.GetFirstConnectedBearer(ctx, modem)
	if err != nil {
		s.Fatal("Error getting Connect APN properties: ", err)
	}

	testing.ContextLog(ctx, "modemAttachApn:", modemAttachApn)
	testing.ContextLog(ctx, "connectApn", connectApn)

	apnName := modemAttachApn["apn"]
	if apnName != expectedLastAttachAPN {
		s.Fatalf("Last Attach APN doesn't match: got %q, want %q", apnName, expectedLastAttachAPN)
	}

	apnName = connectApn["apn"]
	if apnName != expectedLastGoodAPN {
		s.Fatalf("Last good APN doesn't match: got %q, want %q", apnName, expectedLastGoodAPN)
	}
}

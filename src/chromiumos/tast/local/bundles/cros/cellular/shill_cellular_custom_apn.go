// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

type customApnTestParam struct {
	ModbOverrideProto string
	// ExpectedLastAttachAPN string
	// ExpectedLastGoodAPN   string
	// Simulate custom APN from UI
	CustomAPN map[string]interface{}
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularCustomApn,
		Desc: "Verifies that traffic can be sent over the Cellular network when using a custom APN",
		Contacts: []string{
			"andrewlassalle@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr: []string{"group:cellular", "cellular_unstable", "cellular_amari_callbox"},
		Params: []testing.Param{{
			Name:      "attach_apn",
			Val:       customApnTestParam{"callbox_unknown_carrier.pbf", map[string]interface{}{"apn": "callbox-ipv4", "attach": "true"}},
			ExtraData: []string{"callbox_unknown_carrier.pbf"},
		},
			{
				Name:      "attach_apn_username_and_password_default_auth",
				Val:       customApnTestParam{"callbox_unknown_carrier.pbf", map[string]interface{}{"apn": "callbox-ipv4-chap", "attach": "true", "username": "username", "password": "password"}},
				ExtraData: []string{"callbox_unknown_carrier.pbf"},
			}},
		Fixture: "cellular",
		Timeout: 2 * time.Minute,
	})
}

func ShillCellularCustomApn(ctx context.Context, s *testing.State) {
	params := s.Param().(customApnTestParam)
	modbOverrideProto := params.ModbOverrideProto
	// expectedLastGoodAPN := params.ExpectedLastGoodAPN
	// expectedLastAttachAPN := params.ExpectedLastAttachAPN
	customAPN := params.CustomAPN
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
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
	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		s.Fatal("Unable to find Cellular Service for Device: ", err)
	}
	if err = helper.WaitForEnabledState(ctx, true); err != nil {
		s.Fatal("Cellular service did not reach Enabled state: ", err)
	}

	if err = helper.SetAPN(ctx, customAPN); err != nil {
		s.Fatal("Unable to set the custom APN: ", err)
	}

	testing.ContextLog(ctx, "Connecting")
	if isConnected, err := service.IsConnected(ctx); err != nil {
		s.Fatal("Unable to get IsConnected for Service: ", err)
	} else if !isConnected {
		if _, err := helper.ConnectToDefault(ctx); err != nil {
			s.Fatal("Unable to Connect to Service: ", err)
		}
	}

	serviceLastAttachAPN, err := helper.GetCellularLastAttachAPN(ctx)
	if err != nil {
		s.Fatal("Error getting Service properties: ", err)
	}
	serviceLastGoodAPN, err := helper.GetCellularLastGoodAPN(ctx)
	if err != nil {
		s.Fatal("Error getting Service properties: ", err)
	}
	testing.ContextLog(ctx, "serviceLastAttachAPN:", serviceLastAttachAPN)
	testing.ContextLog(ctx, "serviceAPN", serviceLastGoodAPN)

	apn := serviceLastAttachAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
	expectedAPN := customAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
	isAttach := customAPN[shillconst.DevicePropertyCellularAPNInfoApnAttach]
	if isAttach == "true" && apn != expectedAPN {
		s.Fatalf("Last Attach APN doesn't match: got %q, want %q", apn, expectedAPN)
	}

	apn = serviceLastGoodAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
	if apn != expectedAPN {
		s.Fatalf("Last good APN doesn't match: got %q, want %q", apn, expectedAPN)
	}

	// TODO(b/193056754): do some basic connectivity test. Check IP type.
}

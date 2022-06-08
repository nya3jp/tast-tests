// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

type apnTestParam struct {
	ModbOverrideProto     string
	ExpectedLastAttachAPN string
	ExpectedLastGoodAPN   string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularApn,
		Desc: "Verifies that traffic can be sent over the Cellular network",
		Contacts: []string{
			"andrewlassalle@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr: []string{"group:cellular", "cellular_sim_active", "cellular_unstable", "cellular_amari_callbox"},
		Params: []testing.Param{{
			Name:      "round_robin_attach_apn",
			Val:       apnTestParam{"callbox_round_robin_attach.pbf", "callbox-ipv4", "callbox-ipv4"},
			ExtraData: []string{"callbox_round_robin_attach.pbf"},
		}, {
			Name:      "round_robin_connect_ipv4_default_attach",
			Val:       apnTestParam{"callbox_round_robin_connect_ipv4_default_attach.pbf", "callbox-default-attach", "callbox-ipv4"},
			ExtraData: []string{"callbox_round_robin_connect_ipv4_default_attach.pbf"},
		}, {
			Name:      "null_attach_ipv4v6",
			Val:       apnTestParam{"callbox_null_attach_ipv4v6.pbf", "", "callbox-ipv4v6"},
			ExtraData: []string{"callbox_null_attach_ipv4v6.pbf"},
		}, {
			Name:      "null_attach_ipv6",
			Val:       apnTestParam{"callbox_null_attach_ipv6.pbf", "", "callbox-ipv6"},
			ExtraData: []string{"callbox_null_attach_ipv6.pbf"},
		}, {
			Name:      "null_attach_ipv4",
			Val:       apnTestParam{"callbox_null_attach_ipv4.pbf", "", "callbox-ipv4"},
			ExtraData: []string{"callbox_null_attach_ipv4.pbf"},
		}, {
			Name:      "attach_ipv6",
			Val:       apnTestParam{"callbox_attach_ipv6.pbf", "callbox-ipv6", "callbox-ipv6"},
			ExtraData: []string{"callbox_attach_ipv6.pbf"},
		}, {
			Name: "attach_ip_default",
			// Unknown ip_type should fallback to IPv4
			Val:       apnTestParam{"callbox_attach_ip_default.pbf", "callbox-ipv4", "callbox-ipv4"},
			ExtraData: []string{"callbox_attach_ip_default.pbf"},
		}, {
			Name: "attach_authentication_unknown",
			// Unknown authentication should fallback to CHAP
			Val:       apnTestParam{"callbox_attach_auth_unknown.pbf", "callbox-ipv4-chap", "callbox-ipv4-chap"},
			ExtraData: []string{"callbox_attach_auth_unknown.pbf"},
		}, {
			Name:      "attach_authentication_pap",
			Val:       apnTestParam{"callbox_attach_auth_pap.pbf", "callbox-ipv4-pap", "callbox-ipv4-pap"},
			ExtraData: []string{"callbox_attach_auth_pap.pbf"},
		}, {
			Name:      "attach_authentication_chap",
			Val:       apnTestParam{"callbox_attach_auth_chap.pbf", "callbox-ipv4-chap", "callbox-ipv4-chap"},
			ExtraData: []string{"callbox_attach_auth_chap.pbf"},
		}, {
			Name:      "default_attach_different_connect_apn_ipv4",
			Val:       apnTestParam{"callbox_default_attach_different_connect_apn_ipv4.pbf", "callbox-default-attach", "callbox-ipv4"},
			ExtraData: []string{"callbox_default_attach_different_connect_apn_ipv4.pbf"},
		}, {
			Name:      "default_attach_different_connect_apn_ipv4v6",
			Val:       apnTestParam{"callbox_default_attach_different_connect_apn_ipv4v6.pbf", "callbox-default-attach", "callbox-ipv4v6"},
			ExtraData: []string{"callbox_default_attach_different_connect_apn_ipv4v6.pbf"},
		}},
		Fixture: "cellular",
		Timeout: 2 * time.Minute,
	})
}

func ShillCellularApn(ctx context.Context, s *testing.State) {
	params := s.Param().(apnTestParam)
	modbOverrideProto := params.ModbOverrideProto
	expectedLastGoodAPN := params.ExpectedLastGoodAPN
	expectedLastAttachAPN := params.ExpectedLastAttachAPN
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

	apnName := serviceLastAttachAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
	if apnName != expectedLastAttachAPN {
		s.Fatalf("Last Attach APN doesn't match: got %q, want %q", apnName, expectedLastAttachAPN)
	}

	apnName = serviceLastGoodAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
	if apnName != expectedLastGoodAPN {
		s.Fatalf("Last good APN doesn't match: got %q, want %q", apnName, expectedLastGoodAPN)
	}

	// TODO(b/193056754): do some basic connectivity test. Check IP type.
}

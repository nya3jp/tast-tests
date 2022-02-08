// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

type testParam struct {
	ModbOverrideProto     string
	ExpectedLastAttachAPN string
	ExpectedLastGoodAPN   string
	// Configure an Attach APN before starting the test.
	SetInitialAttachAPNValue map[string]interface{}
}

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularApn,
		Desc: "Verifies that traffic can be sent over the Cellular network",
		Contacts: []string{
			"andrewlassalle@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr: []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Params: []testing.Param{{
			Name:      "round_robin_attach_apn",
			Val:       testParam{"callbox_round_robin_attach.pbf", "callbox-ipv4", "callbox-ipv4", nil},
			ExtraData: []string{"callbox_round_robin_attach.pbf"},
		}, {
			Name:      "round_robin_connect_ipv4_default_attach",
			Val:       testParam{"callbox_round_robin_connect_ipv4_default_attach.pbf", "callbox-default-attach", "callbox-ipv4", nil},
			ExtraData: []string{"callbox_round_robin_connect_ipv4_default_attach.pbf"},
		}, {
			Name:      "null_attach_ipv4v6",
			Val:       testParam{"callbox_null_attach_ipv4v6.pbf", "", "callbox-ipv4v6", nil},
			ExtraData: []string{"callbox_null_attach_ipv4v6.pbf"},
		}, {
			Name:      "null_attach_ipv6",
			Val:       testParam{"callbox_null_attach_ipv6.pbf", "", "callbox-ipv6", nil},
			ExtraData: []string{"callbox_null_attach_ipv6.pbf"},
		}, {
			Name:      "null_attach_ipv4",
			Val:       testParam{"callbox_null_attach_ipv4.pbf", "", "callbox-ipv4", nil},
			ExtraData: []string{"callbox_null_attach_ipv4.pbf"},
		}, {
			Name:      "attach_ipv6",
			Val:       testParam{"callbox_attach_ipv6.pbf", "callbox-ipv6", "callbox-ipv6", nil},
			ExtraData: []string{"callbox_attach_ipv6.pbf"},
		}, {
			Name: "attach_ip_default",
			// Unknown authentication should fallback to CHAP
			Val:       testParam{"callbox_attach_ip_default.pbf", "callbox-ipv4", "callbox-ipv4", nil},
			ExtraData: []string{"callbox_attach_ip_default.pbf"},
		}, {
			Name: "attach_authentication_unknown",
			// Unknown authentication should fallback to CHAP
			Val:       testParam{"callbox_attach_auth_unknown.pbf", "callbox-ipv4-chap", "callbox-ipv4-chap", nil},
			ExtraData: []string{"callbox_attach_auth_unknown.pbf"},
		}, {
			Name:      "attach_authentication_pap",
			Val:       testParam{"callbox_attach_auth_pap.pbf", "callbox-ipv4-pap", "callbox-ipv4-pap", nil},
			ExtraData: []string{"callbox_attach_auth_pap.pbf"},
		}, {
			Name:      "attach_authentication_chap",
			Val:       testParam{"callbox_attach_auth_chap.pbf", "callbox-ipv4-chap", "callbox-ipv4-chap", nil},
			ExtraData: []string{"callbox_attach_auth_chap.pbf"},
		}, {
			Name:      "default_attach_different_connect_apn_ipv4",
			Val:       testParam{"callbox_default_attach_different_connect_apn_ipv4.pbf", "callbox-default-attach", "callbox-ipv4", nil},
			ExtraData: []string{"callbox_default_attach_different_connect_apn_ipv4.pbf"},
		}, {
			Name:      "default_attach_different_connect_apn_ipv4v6",
			Val:       testParam{"callbox_default_attach_different_connect_apn_ipv4v6.pbf", "callbox-default-attach", "callbox-ipv4v6", nil},
			ExtraData: []string{"callbox_default_attach_different_connect_apn_ipv4v6.pbf"},
		}},
		Fixture: "cellular",
		Timeout: 2 * time.Minute,
	})
}

func ShillCellularApn(ctx context.Context, s *testing.State) {
	modbOverrideProto := s.Param().(testParam).ModbOverrideProto
	expectedLastGoodAPN := s.Param().(testParam).ExpectedLastGoodAPN
	expectedLastAttachAPN := s.Param().(testParam).ExpectedLastAttachAPN
	setInitialAttachAPNValue := s.Param().(testParam).SetInitialAttachAPNValue
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

	// TODO: is this still used by any tests?
	if setInitialAttachAPNValue != nil {
		modem3gpp, err := modem.GetModem3gpp(ctx)
		if err != nil {
			s.Fatal("Could not get modem3gpp object: ", err)
		}
		if err := modemmanager.SetInitialEpsBearerSettings(ctx, modem3gpp, setInitialAttachAPNValue); err != nil {
			s.Fatal("Failed to set initial EPS bearer settings: ", err)
		}
	}

	if _, err = helper.Disable(ctx); err != nil {
		s.Fatal("Failed to disable cellular: ", err)
	}

	err = cellular.SetServiceProvidersExclusiveOverride(ctx, s.DataPath(modbOverrideProto))
	if err != nil {
		s.Fatal("Failed to set service providers override: ", err)
	}
	defer os.Remove("/usr/share/shill/serviceproviders-exclusive-override.pbf") // TODO: change with constant.
	errs := helper.ResetShill(ctx)                                              //TODO: change array to simple error?
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
	// props, err := service.GetShillProperties(ctx)
	if err != nil {
		s.Fatal("Error getting Service properties: ", err)
	}
	serviceLastGoodAPN, err := helper.GetCellularLastGoodAPN(ctx)
	// props, err := service.GetShillProperties(ctx)
	if err != nil {
		s.Fatal("Error getting Service properties: ", err)
	}
	// serviceLastAttachAPN, err := props.Get(shillconst.ServicePropertyCellularLastAttachAPN)
	// if err != nil {
	// 	s.Fatal("Error getting Service.LastAttachAPN property: ", err)
	// }
	// serviceAPN, err := props.GetStrings(shillconst.ServicePropertyCellularAPN)
	// if err != nil {
	// 	s.Fatal("Error getting Service.APN property: ", err)
	// }
	testing.ContextLog(ctx, "serviceLastAttachAPN:", serviceLastAttachAPN)
	testing.ContextLog(ctx, "serviceAPN", serviceLastGoodAPN)

	apnName := serviceLastAttachAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
	// apn := ""
	// if apnName != nil {
	// 	apn = apnName.(string) // TODO: Is this still needed if we are not using Interface{}
	// }
	if apnName != expectedLastAttachAPN {
		s.Fatalf("last Attach APN doesn't match. Current Attach is %q, expected is %q", apnName, expectedLastAttachAPN)
	}

	apnName = serviceLastGoodAPN[shillconst.DevicePropertyCellularAPNInfoApnName]
	// apn = ""
	// if apnName != nil {
	// 	apn = apnName.(string) // TODO: Is this still needed if we are not using Interface{}
	// }
	if apnName != expectedLastGoodAPN {
		s.Fatalf("last good APN doesn't match. Current APN is %q, expected is %q", apnName, expectedLastGoodAPN)
	}

	// TODO: do some basic connectivity test. Check IP type.
}

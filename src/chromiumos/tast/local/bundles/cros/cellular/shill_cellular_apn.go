// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

type testParam struct {
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
		Data: []string{
			"amari_round_robin_attach.pbf",
		},
		Attr: []string{"group:cellular", "cellular_unstable", "cellular_sim_active"},
		Params: []testing.Param{{
			Name:      "round_robin_attach_apn",
			Val:       testParam{"amari_round_robin_attach.pbf", "amari_ipv4", "amari_ipv4"},
			ExtraData: []string{"amari_round_robin_attach.pbf"},
		}, {
			Name:      "round_robin_connect_ipv4_default_attach",
			Val:       testParam{"amari_round_robin_connect_ipv4_default_attach.pbf", "amari_default_attach", "amari_ipv4"},
			ExtraData: []string{"amari_round_robin_connect_ipv4_default_attach.pbf"},
		}, {
			Name:      "null_attach_ipv6",
			Val:       testParam{"amari_null_attach_ipv6.pbf", "", "amari_ipv6"},
			ExtraData: []string{"amari_null_attach_ipv6.pbf"},
		}, {
			Name:      "null_attach_ipv4",
			Val:       testParam{"amari_null_attach_ipv4.pbf", "", "amari_ipv4"},
			ExtraData: []string{"amari_null_attach_ipv4.pbf"},
		}, {
			Name:      "default_attach_different_connect_apn_ipv4",
			Val:       testParam{"amari_default_attach_different_connect_apn_ipv4.pbf", "amari_default_attach", "amari_ipv4"},
			ExtraData: []string{"amari_default_attach_different_connect_apn_ipv4.pbf"},
		}},
		Fixture: "cellular",
		Timeout: 1 * time.Minute,
	})
}

func resetShill(ctx context.Context) []error {
	var errs []error
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to stop shill"))
	}
	if err := os.Remove(shillconst.DefaultProfilePath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, errors.Wrap(err, "failed to remove default profile"))
	}
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		// No more can be done if shill doesn't start
		return append(errs, errors.Wrap(err, "failed to restart shill"))
	}
	manager, err := shill.NewManager(ctx)
	if err != nil {
		// No more can be done if a manger interface cannot be created
		return append(errs, errors.Wrap(err, "failed to create new shill manager"))
	}
	if err = manager.PopAllUserProfiles(ctx); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to pop all user profiles"))
	}

	// Wait until a service is connected.
	expectProps := map[string]interface{}{
		shillconst.ServicePropertyIsConnected: true,
	}
	if _, err := manager.WaitForServiceProperties(ctx, expectProps, 30*time.Second); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to wait for connected service"))
	}

	return errs
}

func ShillCellularApn(ctx context.Context, s *testing.State) {
	modbOverrideProto := s.Param().(testParam).ModbOverrideProto
	expectedLastGoodAPN := s.Param().(testParam).ExpectedLastGoodAPN
	expectedLastAttachAPN := s.Param().(testParam).ExpectedLastAttachAPN
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	// //TODO:remove
	// serviceLastAttachAPN2, err2 := helper.GetCellularLastAttachAPN(ctx)
	// // props, err := service.GetShillProperties(ctx)
	// if err2 != nil {
	// 	s.Fatal("Error getting Service properties: ", err2)
	// }
	// testing.ContextLog(ctx, "serviceLastAttachAPN:", serviceLastAttachAPN2) //TODO:remove

	if _, err = helper.Disable(ctx); err != nil {
		s.Fatal("Failed to disable cellular: ", err)
	}

	err = cellular.SetServiceProvidersOverride(ctx, s.DataPath(modbOverrideProto))
	if err != nil {
		s.Fatal("Failed to set service providers override: ", err)
	}
	defer os.Remove("/usr/share/shill/serviceproviders-override.pbf") // TODO: change with constant.
	errs := resetShill(ctx)                                           //TODO: change array to simple error?
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
	// if err := helper.ConnectToService(ctx, service); err != nil {
	// 	s.Fatal("Failed to connect to secondary service: ", err)
	// }
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
		s.Fatalf("last good APN doesn't match. Current Attach is %q, expected is %q", apnName, expectedLastGoodAPN)
	}

}

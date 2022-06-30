// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ShillCellularValidateProfile,
		Desc: "Verifies that change in profile property able to connect after shill reset, mimics OS update",
		Contacts: []string{
			"srikanthkumar@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_unstable", "cellular_amari_callbox"},
		Fixture: "cellular",
		Timeout: 5 * time.Minute,
	})
}

func ShillCellularValidateProfile(ctx context.Context, s *testing.State) {
	correctAPNProto := "callbox_attach_ipv4.pbf"
	incorrectAPNProto := "callbox_attach_ipv4_incorrect_apn.pbf"

	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	// Convert default profile to pbf file - Done
	// Create another profile which fails and convert to pbf and load - Done
	// profiles exist at /var/cache/shill/default.profile
	// Load this profile and able to connect successfully to callbox (apn)
	// Load an error profile and check it fails to connect to callbox (apn)
	// ResetShill
	// Should able to connect with default.profile

	deferCleanUp, err := cellular.SetServiceProvidersExclusiveOverride(ctx, s.DataPath(correctAPNProto))
	if err != nil {
		s.Fatal("Failed to set service providers override: ", err)
	}
	defer deferCleanUp()

	// Initial default profile path & properties.
	shillManager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}
	profile, err := shillManager.ProfileByName(ctx, "default")
	if err != nil {
		s.Fatal("Failed to get default profile: ", err)
	}
	s.Log("Default profile path: ", profile)
	prop, err := shillManager.GetProperties(ctx)
	s.Log("Default profile properties: ", prop)

	// Check cellular connection for default profile.
	connected, err := checkCellularConnection(ctx, true)
	if err != nil || !connected {
		s.Fatal("Supposed to connect with the given good apn proto configuration: ", err)
	}

	deferCleanUp, err = cellular.SetServiceProvidersExclusiveOverride(ctx, s.DataPath(incorrectAPNProto))
	if err != nil {
		s.Fatal("Failed to set service providers override: ", err)
	}
	defer deferCleanUp()

	// Check profile properties after modification.
	s.Log("After loading incorrect apn profile")
	shillManager, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("After Failed to create manager proxy: ", err)
	}

	// Check cellular connection, should fail to connect with incorrect apn(ipv4 incorrect one).
	connected, err = checkCellularConnection(ctx, false)
	if connected {
		s.Fatal("Supposed to fail in connecting as incorrect apn")
	}

	// Reset shill to reset previous shill profile settings.
	errs := helper.ResetShill(ctx)
	if errs != nil {
		s.Fatal("Failed to reset shill: ", err)
	}
	testing.ContextLog("Reset Shill")

	// Check cellular connection, should connect with default profile after shill reset.
	connected, err = checkCellularConnection(ctx, true)
	if err != nil || !connected {
		s.Fatal("Supposed to connect with the given good apn proto configuration: ", err)
	}

	serviceLastAttachAPN, err := helper.GetCellularLastAttachAPN(ctx)
	if err != nil {
		s.Fatal("Error getting Service properties: ", err)
	}
	serviceLastGoodAPN, err := helper.GetCellularLastGoodAPN(ctx)
	if err != nil {
		s.Fatal("Error getting Service properties: ", err)
	}
	testing.ContextLog(ctx, "serviceLastAttachAPN: ", serviceLastAttachAPN)
	testing.ContextLog(ctx, "serviceAPN: ", serviceLastGoodAPN)

}

// checkCellularConnection checks for cellular connection and tries to connect if connection 'True'.
func checkCellularConnection(ctx context.Context, connection bool) (bool, error) {
	// Check cellular connection, should able to connect with default apn(ipv4 one).
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to create cellular.Helper")
	}
	if _, err = helper.Enable(ctx); err != nil {
		return false, errors.Wrap(err, "failed to enable cellular")
	}

	// Verify that a connectable Cellular service exists and ensure it is connected.
	service, err := helper.FindServiceForDevice(ctx)
	if err != nil {
		return false, errors.Wrap(err, "unable to find cellular service for device")
	}
	if err = helper.WaitForEnabledState(ctx, true); err != nil {
		return false, errors.Wrap(err, "cellular service did not reach enabled state")
	}

	testing.ContextLog(ctx, "Connecting")
	if isConnected, err := service.IsConnected(ctx); err != nil {
		return false, errors.Wrap(err, "unable to get IsConnected for service")
	} else if !isConnected && connection {
		if _, err := helper.ConnectToDefault(ctx); err != nil {
			return false, errors.Wrap(err, "unable to connect to service")
		}
	}
	return true, nil
}

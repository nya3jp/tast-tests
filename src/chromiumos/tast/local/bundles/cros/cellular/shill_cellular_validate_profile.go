// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
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
	// If needed to use apn, not using as of now
	expectedLastGoodAPN := "callbox-default-attach"
	expectedLastAttachAPN := "callbox-ipv4"
	if _, err := modemmanager.NewModemWithSim(ctx); err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	/*deferCleanUp, err := cellular.SetServiceProvidersExclusiveOverride(ctx, s.DataPath(modbOverrideProto))
	if err != nil {
		s.Fatal("Failed to set service providers override: ", err)
	}
	defer deferCleanUp()*/

	// Steps
	// Read profile property and update with any property that can fail cellular connect
	// Verify no shill serivce connect on cellular
	// Revert/Write good profile property value and connect, check shill service connect on cellular
	// ResetShill (does clears profile data)
	// Check cellular connnection happens with default profile on cellular

	// Get shill profile properties and set 'ProhibitedTechnologies' property with 'cellular'

	// Disable should do technology disable , check is this updating profile file or not else do above way.
	// Observed cellular disabled but profile file not updated.
	if _, err = helper.Disable(ctx); err != nil {
		s.Fatal("Failed to disable cellular: ", err)
	}

	// Initial default profile properties for Enabled, Prohibited technologies keys.
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create manager proxy: ", err)
	}
	profile, err := m.ProfileByName(ctx, "default")
	if err != nil {
		s.Fatal("Failed to get default profile: ", err)
	}
	s.Log("Default profile path: ", profile)
	prop, err := m.GetProperties(ctx)
	technologies, err := prop.GetStrings(shillconst.ManagerPropertyEnabledTechnologies)
	if err != nil {
		s.Fatal("Failed to get enabled technologies: ", err)
	}
	prohhibited, err := prop.GetString(shillconst.ProfilePropertyProhibitedTechnologies)
	if err != nil {
		s.Fatal("Failed to get prohibited technologies: ", err)
	}
	/*
		prohhibited, err := manager.GetEntry(ctx, shillconst.ProfilePropertyProhibitedTechnologies)
		if err != nil {
			s.Fatal("Failed to get default profile prohibited value: ", err)
		}*/
	s.Log("Initial EnabledTechnologies value    : ", technologies)
	s.Log("Initial ProhibitedTechnologies value : ", prohhibited)

	testing.ContextLog(ctx, "Removing cellular technology from portal list")
	if err := m.SetProperty(ctx, shillconst.ProfilePropertyCheckPortalList, "ethernet,wifi"); err != nil {
		s.Fatal("Failed to disable portal detection on cellular: ", err)
	}
	if err := m.SetProperty(ctx, shillconst.ProfilePropertyProhibitedTechnologies, "cellular"); err != nil {
		s.Fatal("Failed to disable cellular: ", err)
	}
	// Check profile properties after modification.
	s.Log("After setting profile property values")
	m, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("After Failed to create manager proxy: ", err)
	}

	prop, err = m.GetProperties(ctx)
	technologies, err = prop.GetStrings(shillconst.ManagerPropertyEnabledTechnologies)
	if err != nil {
		s.Fatal("Failed to get enabled technologies: ", err)
	}
	prohhibited, err = prop.GetString(shillconst.ProfilePropertyProhibitedTechnologies)
	if err != nil {
		s.Fatal("Failed to get prohibited technologies: ", err)
	}
	s.Log("Before reset EnabledTechnologies value    : ", technologies)
	s.Log("Before reset ProhibitedTechnologies value : ", prohhibited)

	// Reset shill to reset previous shill profile settings
	errs := helper.ResetShill(ctx)
	if errs != nil {
		s.Fatal("Failed to reset shill: ", err)
	}

	// Check EnabledTechnologies and ProhibitedTechnologies to reflect default values.
	m, err = shill.NewManager(ctx)
	if err != nil {
		s.Fatal("After Failed to create manager proxy: ", err)
	}
	profile, err = m.ProfileByName(ctx, "default")
	if err != nil {
		s.Fatal("Failed to get default profile: ", err)
	}
	s.Log("After reset profile path: ", profile)
	prop, err = m.GetProperties(ctx)
	technologies, err = prop.GetStrings(shillconst.ManagerPropertyEnabledTechnologies)
	if err != nil {
		s.Fatal("Failed to get enabled technologies: ", err)
	}
	prohhibited, err = prop.GetString(shillconst.ProfilePropertyProhibitedTechnologies)
	if err != nil {
		s.Fatal("Failed to get prohibited technologies: ", err)
	}
	s.Log("After Reset EnabledTechnologies value    : ", technologies)
	s.Log("After Reset ProhibitedTechnologies value : ", prohhibited)

	helper, err = cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("After failed to create new cellular.Helper: ", err)
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
	testing.ContextLog(ctx, "serviceLastAttachAPN: ", serviceLastAttachAPN)
	testing.ContextLog(ctx, "serviceAPN: ", serviceLastGoodAPN)
	// Results: serviceAPN map[apn:jionet apn_source:modb ip_type:ipv4 name:Jio 4G]

}

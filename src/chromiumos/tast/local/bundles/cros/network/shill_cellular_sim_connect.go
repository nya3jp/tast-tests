// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

// This test is only run on the cellular_multisim group. All boards in that group
// provide the Modem.SimSlots property and have at least two provisioned SIM slots.

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularSimConnect,
		Desc:     "Verifies that Shill can connect to a service in a different slot",
		Contacts: []string{"stevenjb@google.com", "cros-network-health@google.com"},
		Attr:     []string{"group:cellular", "cellular_sim_dual_active"},
	})
}

func ShillCellularSimConnect(ctx context.Context, s *testing.State) {
	simProps, primary, err := getModemSimSlots(ctx)
	if err != nil {
		s.Fatal("Failed to get Modem.SimSlots: ", err)
	}
	if simProps[primary] == nil {
		s.Fatalf("No primary SimProperties at %d", primary)
	}
	var secondaryProps *dbusutil.Properties
	var secondary uint32
	for i := uint32(0); i < uint32(len(simProps)); i++ {
		if i == primary {
			continue
		}
		p := simProps[i]
		if p != nil {
			secondary = i
			secondaryProps = p
			break
		}
	}
	if secondaryProps == nil {
		s.Fatal("No secondary SimProperties")
	}

	s.Logf("Primary slot index=%d, Secondary slot index=%d", primary, secondary)

	// Get the secondary slot ICCID and connect to it. This will change the primary slot.
	// Cellular Multisim tests should not rely on a particular slot being active,
	// so we do not defer a slot change if this fails.
	secondaryICCID, err := secondaryProps.GetString(mmconst.SimPropertySimIdentifier)
	if err != nil {
		s.Fatal("Failed to get ICCID: ", err)
	}
	if secondaryICCID == "" {
		s.Fatalf("Empty ICCID for secondary slot: %d", secondary)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper")
	}

	serviceProps := map[string]interface{}{
		shillconst.ServicePropertyCellularICCID: secondaryICCID,
		shillconst.ServicePropertyType:          shillconst.TypeCellular,
	}
	service, err := helper.Manager.WaitForServiceProperties(ctx, serviceProps, shillconst.DefaultTimeout)
	if err != nil {
		s.Fatalf("Cellular Service not found for ICCID: %s: %s", secondaryICCID, err)
	}

	s.Log("Connecting to secondary ICCID: ", secondaryICCID)
	if err := helper.ConnectToService(ctx, service); err != nil {
		s.Fatal("Failed to connect to secondary service: ", err)
	}

	// Connecting to the secondary service will change slots, causing the Modem object to be rebuilt.
	// Request SimSlots properties from the new Modem.
	newSimProps, newPrimary, err := getModemSimSlots(ctx)
	if err != nil {
		s.Fatal("Failed to get Modem.SimSlots: ", err)
	}
	if newPrimary != secondary {
		s.Fatalf("Unexpected primary slot after connect, wanted: %d, got: %d: ", secondary, newPrimary)
	}
	primaryProps := newSimProps[primary]
	if primaryProps == nil {
		s.Fatal("Unexpected nil primary SimProperties")
	}

	// Get the original primary slot ICCID and connect to it.
	primaryICCID, err := primaryProps.GetString(mmconst.SimPropertySimIdentifier)
	if err != nil {
		s.Fatal("Failed to get ICCID: ", err)
	}
	if primaryICCID == "" {
		s.Fatalf("Empty ICCID for primary slot: %d", primary)
	}

	serviceProps[shillconst.ServicePropertyCellularICCID] = primaryICCID
	service, err = helper.Manager.WaitForServiceProperties(ctx, serviceProps, shillconst.DefaultTimeout)
	if err != nil {
		s.Fatalf("Cellular Service not found for ICCID: %s: %s", primaryICCID, err)
	}

	s.Log("Connecting to original primary ICCID=", primaryICCID)
	if err := helper.ConnectToService(ctx, service); err != nil {
		s.Fatal("Failed to connect to primary service: ", err)
	}
}

func getModemSimSlots(ctx context.Context) (simProps []*dbusutil.Properties, primary uint32, err error) {
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to create Modem")
	}
	simProps, primary, err = modem.GetSimSlots(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get SimSlots")
	}
	numSlots := uint32(len(simProps))
	if numSlots < 2 {
		return nil, 0, errors.Errorf("expected at least 2 SIM slots, found: %d", numSlots)
	}
	if primary >= numSlots {
		return nil, 0, errors.Errorf("invalid primary slot, want < %d, got: %d", numSlots, primary)
	}
	return simProps, primary, err
}

// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/local/bundles/cros/network/cellular"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillCellularModemmanager,
		Desc:     "Verifies that Shill behaves correctly when modemmanager is restarted",
		Contacts: []string{"stevenjb@google.com", "chromeos-cellular-team@google.com"},
		Attr:     []string{"group:cellular", "sim_active"},
		Fixture:  "cellular",
	})
}

func ShillCellularModemmanager(ctx context.Context, s *testing.State) {
	modem1, err := modemmanager.NewModem(ctx)
	if err != nil {
		s.Fatal("Failed to create Modem: ", err)
	}

	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}
	if _, err := helper.FindService(ctx); err != nil {
		s.Fatal("Unable to find Cellular Service before modemmanager restart: ", err)
	}

	deviceProps, err := helper.Device.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to get Device properties: ", err)
	}
	modemPath, err := deviceProps.GetString(shillconst.DevicePropertyDBusObject)
	if err != nil {
		s.Fatal("Failed to get Device.DBusObject property: ", err)
	}

	if modemPath != modem1.String() {
		s.Fatalf("Path mismatch, got: %q, want: %q", modemPath, modem1.String())
	}

	if err := upstart.RestartJob(ctx, "modemmanager"); err != nil {
		s.Fatal("Failed to restart modemmanager: ", err)
	}

	var modem2 *modemmanager.Modem
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		modem2, err = modemmanager.NewModem(ctx)
		return err
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: 1 * time.Second,
	}); err != nil {
		s.Fatal("Failed to create Modem after restart: ", err)
	}

	if helper.Device.WaitForProperty(ctx, shillconst.DevicePropertyDBusObject, modem2.String(), 30*time.Second); err != nil {
		s.Fatal("Failed to get matching Device.DBus.Object: ", err)
	}
	if _, err := helper.FindService(ctx); err != nil {
		s.Fatal("Unable to find Cellular Service after modemmanager restart: ", err)
	}
}

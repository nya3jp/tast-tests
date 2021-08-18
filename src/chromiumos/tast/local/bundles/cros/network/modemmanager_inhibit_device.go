// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/modemmanager"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ModemmanagerInhibitDevice,
		Desc: "Verifies that ModemManager1.InhibitDevice succeeds",
		Contacts: []string{
			"stevenjb@google.com",
			"chromeos-cellular-team@google.com",
		},
		Attr:    []string{"group:cellular", "cellular_sim_active"},
		Timeout: 5 * time.Minute,
	})
}

func ModemmanagerInhibitDevice(ctx context.Context, s *testing.State) {
	modem, err := modemmanager.NewModem(ctx)
	if err != nil {
		s.Fatal("Failed to create Modem: ", err)
	}
	props, err := modem.GetProperties(ctx)
	if err != nil {
		s.Fatal("Failed to call GetProperties on Modem: ", err)
	}
	device, err := props.GetString(mmconst.ModemPropertyDevice)
	if err != nil {
		s.Fatal("Missing Device property: ", err)
	}

	for i := 0; i < 3; i++ {
		obj, err := dbusutil.NewDBusObject(ctx, modemmanager.DBusModemmanagerService, modemmanager.DBusModemmanagerInterface, modemmanager.DBusModemmanagerPath)
		if err != nil {
			s.Fatal("Unable to connect to ModemManager1: ", err)
		}
		if err = obj.Call(ctx, "InhibitDevice", device, true).Err; err != nil {
			s.Fatal("InhibitDevice(true) failed: ", err)
		}
		if err = obj.Call(ctx, "InhibitDevice", device, false).Err; err != nil {
			s.Fatal("InhibitDevice(false) failed: ", err)
		}

		modem2, err := modemmanager.NewModem(ctx)
		if err != nil {
			s.Fatal("Failed to create Modem after Un-Inhibit: ", err)
		}
		if modem.ObjectPath() == modem2.ObjectPath() {
			s.Fatalf("Modem path expected to change but did not: %v, err: %s", modem.ObjectPath(), err)
		}
		modem = modem2
	}
}

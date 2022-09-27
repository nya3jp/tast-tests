// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"time"

	"chromiumos/tast/common/mmconst"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/cellular"
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
		Attr:    []string{"group:cellular", "cellular_sim_active", "cellular_ota_avl"},
		Fixture: "cellular",
		Timeout: 5 * time.Minute,
	})
}

func ModemmanagerInhibitDevice(ctx context.Context, s *testing.State) {
	modem, err := modemmanager.NewModemWithSim(ctx)
	if err != nil {
		s.Fatal("Could not find MM dbus object with a valid sim: ", err)
	}
	helper, err := cellular.NewHelper(ctx)
	if err != nil {
		s.Fatal("Failed to create cellular.Helper: ", err)
	}

	cleanupCtx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	defer func(ctx context.Context) {
		// Restart ModemManager after Inhibit test
		if err := helper.RestartModemManager(ctx, true); err != nil {
			s.Fatal("Failed to restart ModemManager: ", err)
		}
	}(cleanupCtx)

	for i := 0; i < 3; i++ {
		props, err := modem.GetProperties(ctx)
		if err != nil {
			s.Fatal("Failed to call GetProperties on Modem: ", err)
		}
		device, err := props.GetString(mmconst.ModemPropertyDevice)
		if err != nil {
			s.Fatal("Missing Device property: ", err)
		}
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

		modem2, err := modemmanager.NewModemWithSim(ctx)
		if err != nil {
			s.Fatal("Failed to create Modem after Un-Inhibit: ", err)
		}
		if modem.ObjectPath() == modem2.ObjectPath() {
			s.Fatalf("Modem path expected to change but did not: %v, err: %s", modem.ObjectPath(), err)
		}
		modem = modem2
	}
}

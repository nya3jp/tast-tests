// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     DBus,
		Desc:     "Demonstrates how to use D-Bus",
		Contacts: []string{"derat@chromium.org", "tast-users@chromium.org"},
	})
}

func DBus(ctx context.Context, s *testing.State) {
	const (
		// Define the D-Bus constants here.
		// Note that this is for the reference only to demonstrate how
		// to use dbusutil. For actual use, session_manager D-Bus call
		// should be performed via
		// chromiumos/tast/local/session_manager pacakge.
		dbusName      = "org.chromium.SessionManager"
		dbusPath      = "/org/chromium/SessionManager"
		dbusInterface = "org.chromium.SessionManagerInterface"

		job = "ui"
	)

	s.Logf("Restarting %s job and waiting for %s service", job, dbusName)
	if err := upstart.RestartJob(ctx, job); err != nil {
		s.Fatalf("Failed to start %s: %v", job, err)
	}
	_, obj, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		s.Fatalf("Failed to connect to %s: %v", dbusName, err)
	}

	s.Log("Asking session_manager for session state")
	var state string
	if err := obj.CallWithContext(ctx, dbusInterface+".RetrieveSessionState", 0).Store(&state); err != nil {
		s.Error("Failed to get session state: ", err)
	} else {
		s.Logf("Session state is %q", state)
	}
}

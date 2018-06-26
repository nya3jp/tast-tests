// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"

	"github.com/godbus/dbus"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DBus,
		Desc: "Demonstrates how to use D-Bus",
		Attr: []string{"informational"},
	})
}

func DBus(s *testing.State) {
	const (
		service = dbusutil.SessionManagerName
		job     = "ui"
	)

	conn, err := dbus.SystemBus()
	if err != nil {
		s.Fatal("failed to connect to system bus: ", err)
	}

	s.Logf("Checking that %s service is already available", service)
	if err = dbusutil.WaitForService(s.Context(), conn, service); err != nil {
		s.Errorf("Failed waiting for %v: %v", service, err)
	}

	s.Logf("Stopping %s job", job)
	if err = upstart.StopJob(job); err != nil {
		s.Errorf("Failed to stop %s: %v", job, err)
	}

	// Start a goroutine that waits for the service and then writes to channel.
	done := make(chan bool)
	go func() {
		if err = dbusutil.WaitForService(s.Context(), conn, service); err != nil {
			s.Errorf("Failed waiting for %v: %v", service, err)
		}
		done <- true
	}()

	s.Logf("Restarting %s job and waiting for %s service", job, service)
	if err = upstart.RestartJob(job); err != nil {
		s.Errorf("Failed to start %s: %v", job, err)
	}
	<-done

	s.Logf("Asking session_manager for session state")
	var state string
	obj := conn.Object(service, dbusutil.SessionManagerPath)
	if err = obj.Call(dbusutil.SessionManagerInterface+".RetrieveSessionState", 0).Store(&state); err != nil {
		s.Errorf("Failed to get session state: %v", err)
	} else {
		s.Logf("Session state is %q", state)
	}
}

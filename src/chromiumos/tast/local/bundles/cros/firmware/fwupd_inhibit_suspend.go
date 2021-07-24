// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FwupdInhibitSuspend,
		Desc: "Ensures .lock file does not exist before, after update, does exist during",
		Contacts: []string{
			"binarynewts@google.org",    // Test Author
			"chromeos-fwupd@google.com", // CrOS FWUPD
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"fwupd"},
	})
}

// RunUpdate updates a fake camera on the DUT
func RunUpdate(ctx context.Context, s *testing.State, finished chan bool) {
	restart := testexec.CommandContext(ctx, "restart", "fwupd")
	if err := restart.Run(testexec.DumpLogOnError); err != nil {
		s.Errorf("%q failed: %v", restart.Args, err)
	}
	s.Log("1) starting update")
	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "update", "-v", "b585990a-003e-5270-89d5-3705a17f9a43")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Errorf("%q failed: %v", cmd.Args, err)
	}
	s.Log("3) finished update")
	finished <- true
}

// FwupdInhibitSuspend runs the fwupdtool utility and makes sure
// that the system can suspend before and after, but not during an update.
func FwupdInhibitSuspend(ctx context.Context, s *testing.State) {
	if _, err := os.Stat("/run/lock/power_override/fwupd.lock"); err == nil {
		s.Fatal("System cannot suspend but no update has started")
	}

	finished := make(chan bool)
	go RunUpdate(ctx, s, finished)

	//TODO: make this time independent (rely either on signals or put a pause in the update)
	testing.Sleep(ctx, 3500*time.Millisecond)
	s.Log("2) checking for file")
	if _, err := os.Stat("/run/lock/power_override/fwupd.lock"); os.IsNotExist(err) {
		s.Fatal("System can suspend but update is in progress")
	}

	<-finished
	if _, err := os.Stat("/run/lock/power_override/fwupd.lock"); err == nil {
		s.Fatal("System cannot suspend but update has finished")
	}
}

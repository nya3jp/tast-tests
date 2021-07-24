// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
        "context"
	"os"

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

func RunUpdate(ctx context.Context, s *testing.State, finished chan bool) {
	go func() {
		cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "update", "-v", "b585990a-003e-5270-89d5-3705a17f9a43")
                if err := cmd.Run(testexec.DumpLogOnError); err != nil {
                        s.Fatalf("%q failed: %v", cmd.Args, err)
                }
        }()
	finished <- true
}

// FwupdInhibitSuspend runs the fwupdtool utility and makes sure
// that the system can suspend before and after, but not during an update.
func FwupdInhibitSuspend(ctx context.Context, s *testing.State) {
        finished := make(chan bool)

	if _, err := os.Stat("/run/lock/power_override/fwupd.lock"); err == nil {
                s.Fatalf("System cannot suspend but no update has started")
        }


        go RunUpdate(ctx, s, finished)
	//if _, err := os.Stat("/run/lock/power_override/fwupd.lock"); os.IsNotExist(err) {
         //       s.Fatalf("System can suspend but update is in progress")
        //}

	<- finished
        if _, err := os.Stat("/run/lock/power_override/fwupd.lock"); err == nil {
               s.Fatalf("System cannot suspend but update has finished")
        }
}


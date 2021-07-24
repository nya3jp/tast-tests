// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"

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

// streamOutput sends back messages as they occur
func streamOutput(rc io.ReadCloser) <-chan string {
	ch := make(chan string)
	r := bufio.NewReader(rc)

	go func() {
		for {
			line, err := r.ReadBytes('\n')
			if s := string(line); s != "" {
				ch <- s
			}
			if err != nil || err == io.EOF {
				break
			}
		}
		rc.Close()
		close(ch)
	}()

	return ch
}

// FwupdInhibitSuspend runs the fwupdtool utility and makes sure
// that the system can suspend before and after, but not during an update.
func FwupdInhibitSuspend(ctx context.Context, s *testing.State) {
	// make sure file does not exist before update
	if _, err := os.Stat("/run/lock/power_override/fwupd.lock"); err == nil {
		s.Fatal("System cannot suspend but no update has started")
	}

	restart := testexec.CommandContext(ctx, "restart", "fwupd")
	if err := restart.Run(testexec.DumpLogOnError); err != nil {
		s.Errorf("%q failed: %v", restart.Args, err)
	}

	// run the update
	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "update", "-v", "b585990a-003e-5270-89d5-3705a17f9a43")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Errorf("%q failed: %v", cmd.Args, err)
	}

	// watch output until update begins write phase
	outch := streamOutput(stdout)
	if err := cmd.Start(); err != nil {
		s.Errorf("%q failed: %v", cmd.Args, err)
	}
	for str := range outch {
		if strings.Contains(str, "Emitting ::status-changed() [device-write]") {
			break
		}
	}
	// ensure that file exists during update
	if _, err := os.Stat("/run/lock/power_override/fwupd.lock"); os.IsNotExist(err) {
		s.Fatal("System can suspend but update is in progress")
	}

	// make sure that file does not exist after update completed
	cmd.Wait()
	if _, err := os.Stat("/run/lock/power_override/fwupd.lock"); err == nil {
		s.Fatal("System cannot suspend but update has finished")
	}
}

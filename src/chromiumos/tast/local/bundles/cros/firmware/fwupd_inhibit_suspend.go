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
	"chromiumos/tast/local/bundles/cros/firmware/fwupd"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
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
		HardwareDeps: hwdep.D(
			hwdep.Battery(),  // Test doesn't run on ChromeOS devices without a battery.
			hwdep.ChromeEC(), // Test requires Chrome EC to set battery to charge via ectool.
		),
	})
}

// streamOutput sends back messages as they occur
func streamOutput(rc io.ReadCloser) <-chan string {
	ch := make(chan string)
	scanner := bufio.NewScanner(rc)
	go func() {
		for scanner.Scan() {
			if s := scanner.Text(); s != "" {
				ch <- s
			}
		}
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

	// make sure dut battery is charging/charged
	if err := fwupd.SetFwupdChargingState(ctx, true); err != nil {
		s.Fatal("Failed to set charging state: ", err)
	}

	// run the update
	cmd := testexec.CommandContext(ctx, "/usr/bin/fwupdmgr", "install", "--allow-reinstall", "-v", fwupd.ReleaseURI)
	cmd.Env = append(os.Environ(), "CACHE_DIRECTORY=/var/cache/fwupd")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Fatalf("%q failed: %v", cmd.Args, err)
	}

	// watch output until update begins write phase
	outch := streamOutput(stdout)
	defer func() {
		for range outch {
		}
	}()

	if err := cmd.Start(); err != nil {
		s.Fatalf("%q failed: %v", cmd.Args, err)
	}
	defer func() {
		cmd.Kill()
		cmd.Wait()
	}()

	// ensure write phase entered; stop reading output at this point
	write := false
	for str := range outch {
		if strings.Contains(str, "Emitting ::status-changed() [device-write]") {
			write = true
			break
		}
	}
	if !write {
		s.Fatal("Write phase not entered by fwupd")
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

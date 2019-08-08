// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     KernelCrash,
		Desc:     "Verify artificial kernel crash creates crash files",
		Contacts: []string{"mutexlox@chromium.org", "cros-monitoring-forensics@google.com"},
		Attr:     []string{"informational"},
	})
}

// waitForNonEmptyGlobsWithTimeout polls the system crash directory until either:
// * for each glob in |globs|, there is exactly one non-empty file that matches the glob and isn't in previous crashes
// * |timeout| expires
func waitForNonEmptyGlobsWithTimeout(ctx context.Context, d *dut.DUT, globs []string, timeout time.Duration, prevCrashes map[string]bool) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		var missingGlobs []string
		var foundFiles []string
		for _, glob := range globs {
			files, err := getMatchingFiles(ctx, d, glob)
			if err != nil {
				return err
			}
			for f := range prevCrashes {
				delete(files, f)
			}
			var filteredFiles []string
			for f := range files {
				filteredFiles = append(filteredFiles, f)
			}
			if len(filteredFiles) == 0 {
				missingGlobs = append(missingGlobs, glob)
			} else if len(filteredFiles) > 1 {
				return errors.Errorf("too many matches for %s: %s", glob, strings.Join(filteredFiles, ", "))
			} else {
				foundFiles = append(foundFiles, filteredFiles[0])
			}
		}
		if len(missingGlobs) != 0 {
			return errors.Errorf("%s not found", strings.Join(missingGlobs, ", "))
		}
		for _, f := range foundFiles {
			if out, err := d.Run(ctx, "rm "+f); err != nil {
				testing.ContextLogf(ctx, "Couldn't rm %s: %s", f, string(out))
			}
		}
		return nil
	}, &testing.PollOptions{Interval: 500 * time.Millisecond, Timeout: timeout})
}

// getMatchingFiles returns a set of files matching the given glob in the
// system crash directory
func getMatchingFiles(ctx context.Context, d *dut.DUT, glob string) (map[string]bool, error) {
	const systemCrashDir = "/var/spool/crash"
	// Use find -print0 instead of ls to handle files with \n in the name.
	cmd := fmt.Sprintf("find %s -mindepth 1 -maxdepth 1 -size +0 -name %s -print0", shutil.Escape(systemCrashDir), shutil.Escape(glob))
	out, err := d.Run(ctx, cmd)
	if err != nil {
		return nil, err
	}
	matches := make(map[string]bool)
	for _, s := range strings.Split(string(out), "\x00") {
		matches[s] = true
	}
	return matches, nil
}

func KernelCrash(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	// Sync filesystem to minimize impact of the panic on other tests
	if _, err := d.Run(ctx, "sync"); err != nil {
		s.Fatal("Failed to sync filesystems")
	}

	// Find any existing kernel crashes so we can ignore them.
	prevCrashes, err := getMatchingFiles(ctx, d, "kernel.*")
	if err != nil {
		s.Fatal("Failed to list existing crash files: ", err)
	}

	// Trigger a panic
	// Run the triggering command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
	cmd := `nohup sh -c 'sleep 2
	if [ -f /sys/kernel/debug/provoke-crash/DIRECT ]; then
		echo PANIC > /sys/kernel/debug/provoke-crash/DIRECT
	else
		echo panic > /proc/breakme
	fi' >/dev/null 2>&1 </dev/null &`
	if _, err := d.Run(ctx, cmd); err != nil {
		s.Fatal("Failed to panic DUT: ", err)
	}

	s.Log("Waiting for DUT to become unreachable")

	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable: ", err)
	}
	s.Log("DUT became unreachable (as expected)")

	s.Log("Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	s.Log("Reconnected to DUT")

	const timeout = time.Second * 30
	globs := []string{"kernel.*.0.bios_log", "kernel.*.0.kcrash", "kernel.*.0.meta"}

	s.Log("Waiting for files to become present")
	if err := waitForNonEmptyGlobsWithTimeout(ctx, d, globs, timeout, prevCrashes); err != nil {
		s.Error("Failed to find crash files: " + err.Error())
	}
}

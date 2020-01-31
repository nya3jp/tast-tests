// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	crash_service "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelCrash,
		Desc:         "Verify artificial kernel crash creates crash files",
		Contacts:     []string{"mutexlox@chromium.org", "cros-monitoring-forensics@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent", "pstore", "reboot"},
		ServiceDeps:  []string{"tast.cros.crash.FixtureService"},
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
			if out, err := d.Command("rm", f).CombinedOutput(ctx); err != nil {
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
	out, err := d.Command("find", systemCrashDir, "-mindepth", "1", "-maxdepth", "1", "-size", "+0", "-name", glob, "-print0").CombinedOutput(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "find failure: %s", out)
	}
	matches := make(map[string]bool)
	for _, s := range strings.Split(string(out), "\x00") {
		matches[s] = true
	}
	return matches, nil
}

func KernelCrash(ctx context.Context, s *testing.State) {
	d := s.DUT()

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	fs := crash_service.NewFixtureServiceClient(cl.Conn)

	if _, err := fs.SetUp(ctx, &empty.Empty{}); err != nil {
		cl.Close(ctx)
		s.Fatal("Failed to set up: ", err)
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// This is a bit delicate. If the test fails _before_ we panic the machine,
	// we need to do TearDown then, and on the same connection (so we can close Chrome).
	//
	// If it fails to reconnect, we do not need to clean these up.
	//
	// Otherwise, we need to re-establish a connection to the machine and
	// run TearDown.
	defer func() {
		if fs != nil {
			if _, err := fs.TearDown(cleanupCtx, &empty.Empty{}); err != nil {
				s.Error("Couldn't tear down: ", err)
			}
		}
		if cl != nil {
			cl.Close(cleanupCtx)
		}
	}()

	if out, err := d.Command("logger", "Running KernelCrash").CombinedOutput(ctx); err != nil {
		s.Logf("WARNING: Failed to log info message: %s", out)
	}

	// Sync filesystem to minimize impact of the panic on other tests
	if out, err := d.Command("sync").CombinedOutput(ctx); err != nil {
		s.Fatalf("Failed to sync filesystems: %s", out)
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
	if err := d.Command("sh", "-c", cmd).Run(ctx); err != nil {
		s.Fatal("Failed to panic DUT: ", err)
	}

	s.Log("Waiting for DUT to become unreachable")

	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable: ", err)
	}
	s.Log("DUT became unreachable (as expected)")

	// When we lost the connection, these connections broke.
	cl.Close(ctx)
	cl = nil
	fs = nil

	s.Log("Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	s.Log("Reconnected to DUT")

	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	fs = crash_service.NewFixtureServiceClient(cl.Conn)

	const timeout = time.Second * 30
	globs := []string{"kernel.*.0.kcrash", "kernel.*.0.meta"}

	s.Log("Waiting for files to become present")
	if err := waitForNonEmptyGlobsWithTimeout(ctx, d, globs, timeout, prevCrashes); err != nil {
		s.Error("Failed to find crash files: " + err.Error())
	}

	s.Log("Tearing down")
}

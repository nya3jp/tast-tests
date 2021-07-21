// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	commoncrash "chromiumos/tast/common/crash"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/dutfs"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/baserpc"
	crashservice "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ReporterStartup,
		Desc: "Verifies crash reporter after reboot",
		Contacts: []string{
			"cros-telemetry@google.com",
			"domlaskowski@chromium.org", // Original autotest author
			"yamaguchi@chromium.org",    // Tast port author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		ServiceDeps: []string{
			"tast.cros.crash.FixtureService",
			"tast.cros.baserpc.FileSystem",
		},
	})
}

func uptime(ctx context.Context, fileSystem baserpc.FileSystemClient) (float64, error) {
	b, err := fileSystem.ReadFile(ctx, &baserpc.ReadFileRequest{Name: "/proc/uptime"})
	if err != nil {
		return 0, errors.Wrap(err, "failed to read uptime file")
	}
	line := string(b.Content)
	data := strings.Split(line, " ")
	if len(data) != 2 {
		return 0, errors.Errorf("unexpected format of uptime file: %q", line)
	}
	uptime, err := strconv.ParseFloat(data[0], 64)
	if err != nil {
		return 0, errors.Wrapf(err, "unexpected content in uptime file: %q", data[0])
	}
	return uptime, nil
}

// ReporterStartup tests crash reporter is set up correctly after reboot.
// Equivalent to the local test in testReporterStartup, but without
// re-initializing to catch problems with the default crash reporting setup.
// See src/chromiumos/tast/local/bundles/cros/platform/user_crash.go
func ReporterStartup(ctx context.Context, s *testing.State) {
	d := s.DUT()

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	fixtureService := crashservice.NewFixtureServiceClient(cl.Conn)

	req := crashservice.SetUpCrashTestRequest{
		Consent: crashservice.SetUpCrashTestRequest_REAL_CONSENT,
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if _, err := fixtureService.SetUp(ctx, &req); err != nil {
		s.Error("Failed to set up: ", err)
		cl.Close(cleanupCtx)
		return
	}

	// This is a bit delicate. If the test fails _before_ we reboot,
	// we need to do TearDown then, and on the same connection (so we can close Chrome).
	//
	// If it fails to reconnect, we do not need to clean these up.
	//
	// Otherwise, we need to re-establish a connection to the machine and
	// run TearDown.
	defer func() {
		if fixtureService != nil {
			if _, err := fixtureService.TearDown(cleanupCtx, &empty.Empty{}); err != nil {
				s.Error("Couldn't tear down: ", err)
			}
		}
		if cl != nil {
			cl.Close(cleanupCtx)
		}
	}()

	// Disable consent.
	consentReq := crashservice.SetConsentRequest{Consent: false}
	if _, err := fixtureService.SetConsent(ctx, &consentReq); err != nil {
		s.Fatal("SetConsent failed: ", err)
	}

	goodCore := false
	defer func() {
		// Restore expected contents if crash_reporter --init didn't work,
		// so that no matter what this test will end with the expected core pattern.
		if !goodCore {
			if err := d.Conn().CommandContext(cleanupCtx, "/bin/bash", "-c", fmt.Sprintf("/bin/echo '%s' > %s", commoncrash.ExpectedCorePattern(), commoncrash.CorePattern)).Run(); err != nil {
				s.Errorf("Failed restoring core pattern file %s: %s", commoncrash.CorePattern, err)
			}
		}
	}()

	fixtureService = nil
	cl.Close(ctx)
	cl = nil

	// On reboot, the core_pattern will be set to |/sbin/crash_reporter --early --log_to_stderr --user=%P:%s:%u:%g:%f
	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	fixtureService = crashservice.NewFixtureServiceClient(cl.Conn)

	// Turn off crash filtering so we see the original setting.
	if _, err := fixtureService.DisableCrashFilter(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to turn off crash filtering: ", err)
	}

	// Check that core_pattern is set up by crash reporter.
	fileSystem := baserpc.NewFileSystemClient(cl.Conn)
	data, err := fileSystem.ReadFile(ctx, &baserpc.ReadFileRequest{Name: commoncrash.CorePattern})
	if err != nil {
		s.Fatal("Failed to read core pattern file: ", commoncrash.CorePattern)
	}
	trimmed := strings.TrimSuffix(string(data.Content), "\n")
	expectedCorePattern := commoncrash.ExpectedCorePattern()
	if trimmed != expectedCorePattern {
		s.Errorf("Unexpected core_pattern: got %s, want %s", trimmed, expectedCorePattern)
	} else {
		goodCore = true
	}

	// Check that we wrote out the file indicating that crash_reporter is
	// enabled AFTER the system was booted. This replaces the old technique
	// of looking for the log message which was flaky when the logs got
	// flooded.
	// NOTE: This technique doesn't need to be highly accurate, we are only
	// verifying that the flag was written after boot and there are multiple
	// seconds between those steps, and a file from a prior boot will almost
	// always have been written out much further back in time than our
	// current boot time.
	fs := dutfs.NewClient(cl.Conn)
	if err := testing.Poll(ctx, func(c context.Context) error {
		flagInfo, err := fs.Stat(ctx, commoncrash.CrashReporterEnabledPath)
		if err != nil {
			return errors.Wrap(err, "failed to open crash reporter enabled file flag")
		} else if !flagInfo.Mode().IsRegular() {
			return errors.Errorf("crash reporter enabled file flag is not a regular file: %s", commoncrash.CrashReporterEnabledPath)
		}
		flagTime := flagInfo.ModTime()

		current := time.Now()
		ut, err := uptime(ctx, fileSystem)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get uptime"))
		}
		// This bootTime can be slightly older than actual. It can theoretically
		// result in a false negative (overlook wrong condition) with very limited
		// timing. However it would be OK practically because boot would take more
		// than a second. Additionally, if bootTime were newer than actual, it may
		// falsely fail when flagTime is right after the actual boot time.
		bootTime := current.Add(time.Duration(-ut * float64(time.Second)))

		// This test depends on the accuracy of the system clock. The clock may be
		// adjusted between system boot and reporter startup, which may make the
		// clock fluctuate by small amount. Make it pass if the difference is
		// small enough compared to the one that happen between reboots.
		if flagTime.Before(bootTime.Add(-2 * time.Second)) {
			return errors.Errorf("user space crash handling was not started during last boot: crash_reporter started at %s, system was booted at %s",
				flagTime.Format(time.RFC3339Nano), bootTime.Format(time.RFC3339Nano))
		}
		return nil
	}, &testing.PollOptions{Timeout: 2 * time.Minute}); err != nil {
		s.Errorf("crash_reporter did not start correctly: %s", err)
	}
}

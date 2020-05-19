// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/binary"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     UncleanShutdownCollector,
		Desc:     "Verify unclean shutdown produces collection",
		Contacts: []string{"joonbug@chromium.org", "cros-telemetry@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func getPersistentInteger(ctx context.Context, file string) (uint64, error) {
	const bytesUint64 = 8 // 8 bytes for uint64
	numUnclean := make([]byte, bytesUint64)

	f, err := os.Open(file)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // On file not exist error, assume count of 0.
		}
		return 0, err
	}

	// Read the persistent integer consisting of uint32 version info and uint64 value.
	// chromium.googlesource.com/chromiumos/platform2/+/HEAD/metrics/persistent_integer.h
	if _, err = f.Seek(4, 0); err != nil { // Skip version information.
		return 0, errors.Wrapf(err, "error while seeking persistent integer file %q", file)
	}

	if _, err := f.Read(numUnclean); err != nil {
		return 0, errors.Wrapf(err, "error while reading persistent integer value %q", file)
	}
	f.Close()

	return binary.LittleEndian.Uint64(numUnclean), nil
}

func UncleanShutdownCollector(ctx context.Context, s *testing.State) {
	const (
		pendingShutdownFile         = "/var/lib/crash_reporter/pending_clean_shutdown"
		uncleanShutdownDetectedFile = "/run/metrics/external/crash-reporter/unclean-shutdown-detected"
		kernelCrashDetectedFile     = "/run/metrics/external/crash-reporter/kernel-crash-detected"
		suspendFile                 = "/var/lib/power_manager/powerd_suspended"
		metricsFile                 = "/var/lib/metrics/Platform.UncleanShutdownsDaily"
		dailyCycleFile              = "/var/lib/metrics/daily.cycle"
	)
	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	if err := upstart.StopJob(ctx, "metrics_daemon"); err != nil {
		s.Fatal("Failed to stop metrics_daemon: ", err)
	}
	defer func() {
		if err := upstart.EnsureJobRunning(ctx, "metrics_daemon"); err != nil {
			s.Error("Failed to re-start metrics_daemon: ", err)
		}
	}()

	oldUnclean, err := getPersistentInteger(ctx, metricsFile)
	if err != nil {
		s.Fatal("Could not get unclean shutdown count: ", err)
	}

	oldDailyCycle, err := getPersistentInteger(ctx, dailyCycleFile)
	if err != nil {
		s.Fatal("Could not get old daily cycle count: ", err)
	}

	// Stash the suspend file so that crash_reporter doesn't see it and
	// assume the unclean shutdown happened while suspended.
	if err := os.Rename(suspendFile, suspendFile+".bak"); err != nil && !os.IsNotExist(err) {
		s.Fatal("Failed to stash suspendFile: ", err)
	} else if err == nil {
		defer func() {
			if err := os.Rename(suspendFile+".bak", suspendFile); err != nil {
				s.Error("Failed to restore suspendFile: ", err)
			}
		}()
	}

	// crash_reporter sees the existing pending_clean_shutdown file (which
	// is created on boot), creates the unclean shutdown file, and then
	// ensures that the pending_clean_shutdown file exists.
	if err := testexec.CommandContext(ctx, "/sbin/crash_reporter", "--boot_collect").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Could not run crash reporter: ", err)
	}

	if _, err = os.Stat(uncleanShutdownDetectedFile); err != nil {
		// If crash_reporter detected a kernel crash it will instead touch the
		// kernelCrashDetectedFile. This isn't ideal, but it can happen
		// for reasons out of this test's control (e.g. if the last boot
		// happened to have a BIOS panic).

		// TODO(https://crbug.com/1083533): Make this a warning so it isn't as silent.
		s.Log("unclean_shutdown_collector failed to create unclean shutdown file: ", err)
		if _, err := os.Stat(kernelCrashDetectedFile); err != nil {
			// Neither file was created -- that's not supposed to happen.
			s.Fatal("unclean_shutdown_collector failed to create either unclean-shutdown-detected or kernel-crash-detected: ", err)
		}
		// As a last-ditch attempt to verify that metrics daemon works, create the file manually.
		if err := ioutil.WriteFile(uncleanShutdownDetectedFile, []byte(""), 0644); err != nil {
			s.Fatalf("Failed to manually create %q: %v", uncleanShutdownDetectedFile, err)
		}
	}
	if _, err = os.Stat(pendingShutdownFile); err != nil {
		s.Fatal("crash_reporter failed to re-create pending shutdown file: ", err)
	}

	if err := upstart.StartJob(ctx, "metrics_daemon"); err != nil {
		s.Fatal("Upstart couldn't restart metrics_daemon: ", err)
	}

	// Wait for unclean shutdown count to be updated.
	if err := testing.Poll(ctx, func(c context.Context) error {
		newUnclean, err := getPersistentInteger(ctx, metricsFile)
		if err != nil {
			return errors.Wrap(err, "could not get unclean shutdown count")
		}

		newDailyCycle, err := getPersistentInteger(ctx, dailyCycleFile)
		if err != nil {
			return errors.Wrap(err, "could not get daily cycle count: ")
		}

		// The count should either increment, or we should move to the next day and set the count to 1.
		if newUnclean == oldUnclean+1 || (newDailyCycle == oldDailyCycle+1 && newUnclean == 1) {
			return nil
		}
		return errors.Errorf("Did not see unclean shutdown. got {unclean: %d, cycle: %d}, want {unclean: %d, cycle: %d} or {unclean: 1, cycle: %d}",
			newUnclean, newDailyCycle, oldUnclean+1, oldDailyCycle, newDailyCycle)
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Unclean shutdown was logged incorrectly: ", err)
	}

	// Also ensure that uncleanShutdownDetectedFile is deleted so that
	// metrics_daemon doesn't repeatedly consume it.
	if _, err := os.Stat(uncleanShutdownDetectedFile); !os.IsNotExist(err) {
		s.Error("Unclean shutdown file was not removed: ", err)
	}
}

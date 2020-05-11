// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"encoding/binary"
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

func getUncleanShutdownCount(ctx context.Context) (uint64, error) {
	const metricsFile = "/var/lib/metrics/Platform.UncleanShutdownsDaily"
	const bytesUint64 = 8 // 8 bytes for uint64
	numUnclean := make([]byte, bytesUint64)

	f, err := os.Open(metricsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // On file not exist error, assume count of 0.
		}
		return 0, err
	}

	// Read the persistent integer consisting of uint32 version info and uint64 value.
	// chromium.googlesource.com/chromiumos/platform2/+/HEAD/metrics/persistent_integer.h
	if _, err = f.Seek(4, 0); err != nil { // Skip version information.
		return 0, errors.Wrap(err, "Error while seeking unclean shutdown file")
	}

	if _, err := f.Read(numUnclean); err != nil {
		return 0, errors.Wrap(err, "Error while reading unclean shutdown count")
	}
	f.Close()

	return binary.LittleEndian.Uint64(numUnclean), nil
}

func UncleanShutdownCollector(ctx context.Context, s *testing.State) {
	const (
		pendingShutdownFile         = "/var/lib/crash_reporter/pending_clean_shutdown"
		uncleanShutdownDetectedFile = "/run/metrics/external/crash-reporter/unclean-shutdown-detected"
		suspendFile                 = "/var/lib/power_manager/powerd_suspended"
	)
	if err := crash.SetUpCrashTest(ctx, crash.WithMockConsent()); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest(ctx)

	oldUnclean, err := getUncleanShutdownCount(ctx)
	if err != nil {
		s.Fatal("Could not get unclean shutdown count: ", err)
	}

	s.Log("Current unclean count: ", oldUnclean)

	if err := upstart.StopJob(ctx, "metrics_daemon"); err != nil {
		s.Fatal("Failed to stop metrics_daemon: ", err)
	}
	defer func() {
		if err := upstart.EnsureJobRunning(ctx, "metrics_daemon"); err != nil {
			s.Error("Failed to re-start metrics_daemon: ", err)
		}
	}()

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
		s.Fatal("unclean_shutdown_collector failed to create unclean shutdown file: ", err)
	}
	if _, err = os.Stat(pendingShutdownFile); err != nil {
		s.Fatal("crash_reporter failed to re-create pending shutdown file: ", err)
	}

	if err := upstart.StartJob(ctx, "metrics_daemon"); err != nil {
		s.Fatal("Upstart couldn't restart metrics_daemon: ", err)
	}

	// Wait for unclean shutdown count to be updated.
	if err := testing.Poll(ctx, func(c context.Context) error {
		newUnclean, err := getUncleanShutdownCount(ctx)
		if err != nil {
			return errors.Wrap(err, "could not get unclean shutdown count")
		}

		if newUnclean != oldUnclean+1 {
			return errors.Errorf("Did not see unclean shutdown. Got %d but expected %d", newUnclean, oldUnclean+1)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Error("Unclean shutdown was logged incorrectly: ", err)
	}

	// Also ensure that uncleanShutdownDetectedFile is deleted so that
	// metrics_daemon doesn't repeatedly consume it.
	if _, err := os.Stat(uncleanShutdownDetectedFile); !os.IsNotExist(err) {
		s.Error("Unclean shutdown file was not removed: ", err)
	}
}

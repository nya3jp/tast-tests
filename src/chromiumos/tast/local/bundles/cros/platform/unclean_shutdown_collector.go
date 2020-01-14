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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UncleanShutdownCollector,
		Desc:         "Verify unclean shutdown produces collection",
		Contacts:     []string{"joonbug@chromium.org", "cros-monitoring-forensics@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		Pre:          chrome.LoggedIn(),
	})
}

func getUncleanShutdownCount(ctx context.Context) (uint64, error) {
	const metricsFile = "/var/lib/metrics/Platform.UncleanShutdownsDaily"
	numUnclean := make([]byte, 8) // 8 bytes for uint64

	f, err := os.Open(metricsFile)
	if err != nil {
		return 0, err
	}

	// Read the persistent integer consisting of uint32 version info and uint64 value.
	// chromium.googlesource.com/chromiumos/platform2/+/HEAD/metrics/persistent_integer.h
	f.Seek(4, 0) // Skip version information.
	f.Read(numUnclean)
	f.Close()

	return binary.LittleEndian.Uint64(numUnclean), nil
}

func UncleanShutdownCollector(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		s.Fatal("SetUpCrashTest failed: ", err)
	}
	defer crash.TearDownCrashTest()

	const uncleanShutdownDetectedFile = "/run/metrics/external/crash-reporter/unclean-shutdown-detected"

	oldUnclean, err := getUncleanShutdownCount(ctx)
	if err != nil {
		s.Fatal("Could not get unclean shutdown count: ", err)
	}
	s.Log("Current unclean count: ", oldUnclean)

	// Create uncleanShutdownDetectedFile to simulate an unclean shutdown.
	_, err = os.Stat(uncleanShutdownDetectedFile)
	if os.IsNotExist(err) {
		var f, err = os.Create(uncleanShutdownDetectedFile)
		if err != nil {
			s.Fatal("Failed to fake an unclean shutdown: ", err)
		}

		f.Close()
	}

	s.Log("Restarting metrics_daemon")
	cmd := testexec.CommandContext(ctx, "pkill", "metrics_daemon")
	if err := cmd.Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to restart metrics_daemon: ", err)
	}

	// Wait for uncleanShutdownDetectedFile to be consumed by metrics daemon
	if err := testing.Poll(ctx, func(c context.Context) error {
		// Check if file exists
		_, err := os.Stat(uncleanShutdownDetectedFile)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return errors.New("Unclean shutdown file is still there")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Could not wait for unclean shutdown to be detected: ", err)
	}

	newUnclean, err := getUncleanShutdownCount(ctx)
	if err != nil {
		s.Fatal("Could not get unclean shutdown count: ", err)
	}

	if newUnclean != oldUnclean+1 {
		s.Fatalf("Unclean shutdown was logged incorrectly. Got %d but expected %d", newUnclean, oldUnclean+1)
	}
}

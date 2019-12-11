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
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     UncleanShutdownCollector,
		Desc:     "Verify unclean shutdown produces collection",
		Contacts: []string{"joonbug@chromium.org", "cros-monitoring-forensics@google.com"},
		Attr:     []string{"informational"},
	})
}

func getUncleanShutdownCount(ctx context.Context, s *testing.State) uint64 {
	const metricsFile = "/var/lib/metrics/Platform.UncleanShutdownsDaily"
	numUnclean := make([]byte, 8)

	f, err := os.Open(metricsFile)
	if err != nil {
		s.Fatal("Failed to open metrics file: ", err)
	}

	f.Seek(4, 0)
	f.Read(numUnclean)
	f.Close()

	return binary.LittleEndian.Uint64(numUnclean)
}

func UncleanShutdownCollector(ctx context.Context, s *testing.State) {
	const uncleanShutdownDetectedFile = "/run/metrics/external/crash-reporter/unclean-shutdown-detected"

	unclean := getUncleanShutdownCount(ctx, s)
	s.Log("Current unclean count: ", unclean)

	// Create uncleanShutdownDetectedFile to simulate an unclean shutdown.
	var _, err = os.Stat(uncleanShutdownDetectedFile)
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
		// check if file exists
		var _, err = os.Stat(uncleanShutdownDetectedFile)

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

	newUnclean := getUncleanShutdownCount(ctx, s)

	if newUnclean != unclean+1 {
		s.Fatal("Unclean shutdown was logged incorrectly. Count should be ", unclean+1, " but got ", newUnclean)
	}
}

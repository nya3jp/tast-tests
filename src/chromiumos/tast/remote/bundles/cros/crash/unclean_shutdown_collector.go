// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"encoding/binary"
	"io/ioutil"
	"os"

	"chromiumos/tast/dut"
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

func uncleanShutdownCount(ctx context.Context, d *dut.DUT, s *testing.State) uint64 {
	const metricsFile = "/var/lib/metrics/Platform.UncleanShutdownsDaily"
	const tempFile = "/tmp/unclean_shutdown_count"

	out, err := d.Command("sudo", "cat", metricsFile).CombinedOutput(ctx)
	if err != nil {
		s.Log(err)
	}
	s.Log("read", out)

	if err := ioutil.WriteFile(tempFile, out, 0644); err != nil {
		s.Log(err)
	}

	f, err := os.Open(tempFile)
	f.Seek(4, 0)
	numUnclean := make([]byte, 8)
	f.Read(numUnclean)
	s.Log(numUnclean)
	os.Remove(tempFile)
	return binary.BigEndian.Uint64(numUnclean)
}

func UncleanShutdownCollector(ctx context.Context, s *testing.State) {
	d, ok := dut.FromContext(ctx)
	if !ok {
		s.Fatal("Failed to get DUT")
	}

	if out, err := d.Command("logger", "Running UncleanShutdownCollector").CombinedOutput(ctx); err != nil {
		s.Logf("WARNING: Failed to log info message: %s", out)
	}

	// Sync filesystem to minimize impact of the unclean shutdown on other tests
	if out, err := d.Command("sync").CombinedOutput(ctx); err != nil {
		s.Fatalf("Failed to sync filesystems: %s", out)
	}

	unclean := uncleanShutdownCount(ctx, d, s)
	s.Log(unclean)

	// Perform a unclean shutdown
	if err := d.Command("sh", "-c", "reboot", "--force").Run(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	s.Log("Waiting for DUT to become unreachable")
	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable: ", err)
	}

	s.Log("Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}

	s.Log("Connected, reading unclean shutdown count")
	newUnclean := uncleanShutdownCount(ctx, d, s)
	s.Log(unclean)

	if newUnclean != unclean+1 {
		s.Fatal("Unclean shutdown wasn't logged: ", newUnclean, " != ", unclean, " +1")
	}
}

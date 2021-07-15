// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrasRecord,
		Desc:         "Verifies CRAS record function works correctly",
		Contacts:     []string{"yuhsuan@chromium.org", "cychiang@chromium.org"},
		HardwareDeps: hwdep.D(hwdep.Microphone()),
		Attr:         []string{"group:mainline", "informational"},
	})
}

func CrasRecord(ctx context.Context, s *testing.State) {
	const duration = 5 // second

	if err := audio.WaitForDevice(ctx, audio.InputStream); err != nil {
		s.Fatal("Failed to wait for input stream: ", err)
	}

	// Set timeout to duration + 1s, which is the time buffer to complete the normal execution.
	runCtx, cancel := context.WithTimeout(ctx, (duration+1)*time.Second)
	defer cancel()

	// Record function by CRAS.
	command := testexec.CommandContext(
		runCtx, "cras_test_client",
		"--capture_file", "/dev/null",
		"--duration", strconv.Itoa(duration),
		"--num_channels", "2",
		"--rate", "48000")
	command.Start()

	defer func() {
		if err := command.Wait(); err != nil {
			s.Fatal("Record did not finish in time: ", err)
		}
	}()

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.InputStream)
	if err != nil {
		s.Fatal("Failed to detect running input device: ", err)
	}

	s.Log("Input device: ", devName)

	if strings.Contains(devName, "Silent") {
		s.Fatal("Fallback to the silent device")
	}
}

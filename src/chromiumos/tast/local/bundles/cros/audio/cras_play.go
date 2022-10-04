// Copyright 2019 The ChromiumOS Authors
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
		Func:         CrasPlay,
		Desc:         "Verifies CRAS playback function works correctly",
		Contacts:     []string{"yuhsuan@chromium.org", "cychiang@chromium.org"},
		HardwareDeps: hwdep.D(hwdep.Speaker()),
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			// TODO(b/244254621) : remove "sasukette" when b/244254621 is fixed.
			ExtraSoftwareDeps: []string{"audio_stable"},
			ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel("sasukette")),
		}, {
			Name:              "unstable_platform",
			ExtraSoftwareDeps: []string{"audio_unstable"},
			ExtraAttr:         []string{"informational"},
		}, {
			Name:              "unstable_model",
			ExtraHardwareDeps: hwdep.D(hwdep.Model("sasukette")),
			ExtraAttr:         []string{"informational"},
		}},
	})
}

func CrasPlay(ctx context.Context, s *testing.State) {
	const duration = 5 // second

	if err := audio.WaitForDevice(ctx, audio.OutputStream); err != nil {
		s.Fatal("Failed to wait for output stream: ", err)
	}

	// Set timeout to duration + 1s, which is the time buffer to complete the normal execution.
	runCtx, cancel := context.WithTimeout(ctx, (duration+1)*time.Second)
	defer cancel()

	// Playback function by CRAS.
	command := testexec.CommandContext(
		runCtx, "cras_test_client",
		"--playback_file", "/dev/zero",
		"--duration", strconv.Itoa(duration),
		"--num_channels", "2",
		"--rate", "48000")
	command.Start()

	defer func() {
		if err := command.Wait(); err != nil {
			s.Fatal("Playback did not finish in time: ", err)
		}
	}()

	devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
	if err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	s.Log("Output device: ", devName)

	if strings.Contains(devName, "Silent") {
		s.Fatal("Fallback to the silent device")
	}
}

// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrasPlay,
		Desc:         "Verifies CRAS playback function works correctly",
		Contacts:     []string{"yuhsuan@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"audio_play"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func CrasPlay(ctx context.Context, s *testing.State) {

	var devName string

	// Get the first running output device by parsing audio thread logs.
	// A divice may not be opened immediately so it will repeat a query until there is a running output device.
	getRunningOutputDevice := func(ctx context.Context) error {
		s.Log("Dump audio thread to check running devices")
		dump, err := testexec.CommandContext(ctx, "cras_test_client", "--dump_audio_thread").Output()
		if err != nil {
			return errors.Errorf("failed to dump audio thread: %s", err)
		}

		re, _ := regexp.Compile("Output dev: (.*)")
		dev := re.FindStringSubmatch(string(dump))
		if dev != nil {
			devName = dev[1]
			return nil
		}
		return errors.New("no such device")
	}

	// Playback function by CRAS.
	command := testexec.CommandContext(
		ctx, "cras_test_client",
		"--playback_file", "/dev/zero",
		"--duration", "1",
		"--num_channels", "2",
		"--rate", "48000")
	command.Start()

	if err := testing.Poll(ctx, getRunningOutputDevice, &testing.PollOptions{Timeout: 1 * time.Second}); err != nil {
		s.Fatal("Failed to detect running output device: ", err)
	}

	s.Log("Output device: ", devName)

	if strings.Contains(devName, "Silent") {
		s.Fatal("Fallback to the silent device")
	}

	if err := command.Wait(); err != nil {
		s.Fatal("Playback did not finish in time: ", err)
	}
}

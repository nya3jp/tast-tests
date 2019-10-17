// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrasRecord,
		Desc:         "Verifies CRAS record function works correctly",
		Contacts:     []string{"yuhsuan@chromium.org", "cychiang@chromium.org"},
		SoftwareDeps: []string{"audio_record"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func CrasRecord(ctx context.Context, s *testing.State) {

	// Return the first running input device by parsing audio thread logs.
	// A divice may not be opened immediately so it will repeat a query until there is a running input device.
	getRunningInputDevice := func(ctx context.Context, timeout time.Duration) string {
		ctx, cancel := ctxutil.OptionalTimeout(ctx, timeout)
		defer cancel()
		for {
			s.Log("Dump audio thread to check running devices")
			dump, err := testexec.CommandContext(ctx, "cras_test_client", "--dump_audio_thread").Output()
			if err != nil {
				s.Fatal("Failed to dump audio thread: ", err)
			}

			re, _ := regexp.Compile("Input dev: (.*)")
			dev := re.FindStringSubmatch(string(dump))
			if dev != nil {
				return dev[1]
			}

			select {
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				s.Fatal("Failed to detect running input device")
			}
		}
	}

	// Playback function by CRAS.
	command := testexec.CommandContext(
		ctx, "cras_test_client",
		"--capture_file", "/dev/null",
		"--duration", "1",
		"--num_channels", "2",
		"--rate", "48000")
	command.Start()

	dev := getRunningInputDevice(ctx, time.Second)
	s.Log("Input device: ", dev)

	if strings.Contains(dev, "Silent") {
		s.Fatal("Fallback to the silent device")
	}

	if err := command.Wait(); err != nil {
		s.Fatal("Record did not finish in time: ", err)
	}
}

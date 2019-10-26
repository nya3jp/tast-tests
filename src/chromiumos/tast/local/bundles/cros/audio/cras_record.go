// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
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
	const (
		duration         = 1 // second
		getDeviceTimeout = 1 * time.Second
	)

	var devName string

	if err := audio.WaitForDevice(ctx, audio.InputStream); err != nil {
		s.Fatal("Failed to wait for input stream: ", err)
	}

	// Get the first running input device by parsing audio thread logs.
	// A device may not be opened immediately so it will repeat a query until there is a running input device.
	re := regexp.MustCompile("Input dev: (.*)")
	getRunningInputDevice := func(ctx context.Context) error {
		s.Log("Dump audio thread to check running devices")
		dump, err := testexec.CommandContext(ctx, "cras_test_client", "--dump_audio_thread").Output()
		if err != nil {
			return errors.Errorf("failed to dump audio thread: %s", err)
		}

		dev := re.FindStringSubmatch(string(dump))
		if dev != nil {
			devName = dev[1]
			return nil
		}
		return errors.New("no such device")
	}

	// Set timeout to duration + 1s, which is the time buffer to complete the normal execution.
	runCtx, cancel := ctxutil.OptionalTimeout(ctx, (duration+1)*time.Second)
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

	if err := testing.Poll(ctx, getRunningInputDevice, &testing.PollOptions{Timeout: getDeviceTimeout}); err != nil {
		s.Fatal("Failed to detect running input device: ", err)
	}

	s.Log("Input device: ", devName)

	if strings.Contains(devName, "Silent") {
		s.Fatal("Fallback to the silent device")
	}
}

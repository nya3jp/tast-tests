// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"bufio"
	"bytes"
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/pre"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EMMCWriteload,
		Desc:         "Continuous test which runs chromeos-install in a loop",
		Contacts:     []string{"js@semihalf.com", "chromeos-firmware@google.com"},
		Attr:         []string{"group:firmware", "firmware_experimental"},
		Pre:          pre.UsbDevMode(),
		Data:         []string{firmware.ConfigFile},
		ServiceDeps:  []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		SoftwareDeps: []string{"crossystem", "flashrom"},
		Vars:         []string{"servo"},
		HardwareDeps: hwdep.D(),
	})
}

func EMMCWriteload(ctx context.Context, s *testing.State) {
	const (
		TestDuration = 240 * time.Minute
	)
	endTime := time.Now().Add(TestDuration)
	warningRegex := regexp.MustCompile(`mmc[0-9]+: Timeout waiting for hardware interrupt`)

	h := s.PreValue().(*pre.Value).Helper

	// Spawn chromeOS installation as many times as we can within
	// the test duration. Single install takes about 3-5 minutes
	// the actual duration might be a little bit longer, but this
	// should not be a problem.
	for time.Now().Before(endTime) {
		s.Log("Installing Chrome OS")
		startTime := time.Now()
		out, err := h.DUT.Conn().CommandContext(ctx, "/usr/sbin/chromeos-install", "--yes").Output()
		if err != nil {
			s.Log("== chromeos-install output: ==\n", string(out))
			s.Fatalf("Error during Chrome OS installation: %s", err)
		}
		elapsedTime := time.Since(startTime)
		s.Logf("Chrome OS installation took %s", elapsedTime)

		dmesg, err := h.DUT.Conn().Command("dmesg").Output(ctx)
		if err != nil {
			s.Fatalf("Failed reading dmesg: %s", err)
		}
		reader := bytes.NewReader(dmesg)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			match := warningRegex.FindAllString(line, -1)
			if len(match) >= 1 {
				s.Fatalf("Found eMMC error in dmesg: %s", line)
			}
		}
	}
}

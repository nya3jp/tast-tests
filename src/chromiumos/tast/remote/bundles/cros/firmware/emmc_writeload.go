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

	"chromiumos/tast/common/testexec"
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
		Pre:          pre.USBDevMode(),
		Data:         pre.Data,
		ServiceDeps:  pre.ServiceDeps,
		SoftwareDeps: pre.SoftwareDeps,
		Vars:         append([]string{"firmware.reinstall_time"}, pre.Vars...),
		HardwareDeps: hwdep.D(),
		Timeout:      260 * time.Minute, // 4hrs 20mins, added 20mins for the last run
	})
}

func EMMCWriteload(ctx context.Context, s *testing.State) {
	testDuration := 240 * time.Minute
	if v, ok := s.Var("firmware.reinstall_time"); ok {
		var err error
		if testDuration, err = time.ParseDuration(v); err != nil {
			s.Fatalf("Failed to parse duration %s: %v", v, err)
		}
	}
	endTime := time.Now().Add(testDuration)
	warningRegex := regexp.MustCompile(`mmc[0-9]+: Timeout waiting for hardware interrupt`)

	h := s.PreValue().(*pre.Value).Helper

	// Spawn chromeOS installation as many times as we can within
	// the test duration. Single install takes about 3-5 minutes
	// the actual duration might be a little bit longer, but this
	// should not be a problem.
	s.Logf("Installing Chrome OS continuously in a loop, should take ~%s", testDuration)
	for time.Now().Before(endTime) {
		if err := h.DUT.Conn().CommandContext(ctx, "/usr/sbin/chromeos-install", "--yes").Run(testexec.DumpLogOnError); err != nil {
			s.Fatalf("Error during Chrome OS installation: %s", err)
		}

		dmesg, err := h.DUT.Conn().CommandContext(ctx, "dmesg", "-cT").Output()
		if err != nil {
			s.Fatalf("Failed reading dmesg after chromeos-install: %s", err)
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

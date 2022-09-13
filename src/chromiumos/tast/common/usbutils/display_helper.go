// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package usbutils

import (
	"context"
	"io/ioutil"
	"regexp"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// externalDisplayDetection verifies connected extended display is detected or not.
func externalDisplayDetection(ctx context.Context, dut *dut.DUT, numberOfDisplays int, regexpPatterns []*regexp.Regexp, remoteTest bool) error {
	displayInfoFile := "/sys/kernel/debug/dri/0/i915_display_info"
	displayInfo := regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	var out []byte
	var err error
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if remoteTest {
			out, err = linuxssh.ReadFile(ctx, dut.Conn(), displayInfoFile)
		} else {
			out, err = ioutil.ReadFile(displayInfoFile)
		}
		if err != nil {
			return errors.Wrap(err, "failed to run display info command")
		}

		matchedString := displayInfo.FindAllString(string(out), -1)
		if len(matchedString) != numberOfDisplays {
			return errors.New("connected external display info not found")
		}

		for _, pattern := range regexpPatterns {
			if !(pattern).MatchString(string(out)) {
				return errors.Errorf("failed %q error message", pattern)
			}
		}

		return nil
	}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
		return errors.Wrap(err, "please connect external display as required")
	}
	return nil
}

// ExternalDisplayDetectionForLocal verifies connected extended display is detected or not for local tests.
func ExternalDisplayDetectionForLocal(ctx context.Context, numberOfDisplays int, regexpPatterns []*regexp.Regexp) error {
	return externalDisplayDetection(ctx, nil, numberOfDisplays, regexpPatterns, false)
}

// ExternalDisplayDetectionForRemote verifies connected extended display is detected or not for remote tests.
func ExternalDisplayDetectionForRemote(ctx context.Context, dut *dut.DUT, numberOfDisplays int, regexpPatterns []*regexp.Regexp) error {
	return externalDisplayDetection(ctx, dut, numberOfDisplays, regexpPatterns, true)
}

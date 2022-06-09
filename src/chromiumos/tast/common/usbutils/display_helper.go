// Copyright 2022 The ChromiumOS Authors.
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

// ExternalDisplayDetection verifies connected extended display is detected or not.
//
// For local-side dut parameter must be nil.
// Example:
// usbDeviceInfo, err := usbutils.ExternalDisplayDetection(ctx, nil, 1, regexpPattern){
// ...
//
// For remote-side dut parameter must be non-nil.
// Example:
// dut := s.DUT()
// usbDeviceInfo, err := usbutils.ExternalDisplayDetection(ctx, dut, 1, regexpPattern){
// ...
func ExternalDisplayDetection(ctx context.Context, dut *dut.DUT, numberOfDisplays int, regexpPatterns []*regexp.Regexp) error {
	displayInfoFile := "/sys/kernel/debug/dri/0/i915_display_info"
	displayInfo := regexp.MustCompile(`.*pipe\s+[BCD]\]:\n.*active=yes, mode=.[0-9]+x[0-9]+.: [0-9]+.*\s+[hw: active=yes]+`)
	var out []byte
	var err error
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if dut != nil {
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

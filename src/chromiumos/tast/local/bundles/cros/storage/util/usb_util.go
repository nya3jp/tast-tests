// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"regexp"
	"strconv"

	"chromiumos/tast/common/storage"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	// Healthy means that the device does not indicate failure or limited remaining life time.
	Healthy storage.LifeStatus = iota
	// Failing indicates the storage device failed or will soon.
	Failing
	// NotApplicable indicates the storage device does not support SMART.
	NotApplicable
)

// RunSmartHealth runs the smartctl command to get health status
func RunSmartHealth(ctx context.Context, device string) ([]byte, error) {
	command := "smartctl -H " + device
	out, err := testexec.CommandContext(ctx, "sh", "-c", command).Output(testexec.DumpLogOnError)
	if err != nil {
		return out, err
	}
	return out, nil
}

// ParseUsbHealth parses the output of smartctl
func ParseUsbHealth(ctx context.Context, outLines []string) storage.LifeStatus {
	usbPassed := regexp.MustCompile(`.*SMART overall-health self-assessment test result: (?P<result>\S*)`)

	for _, line := range outLines {
		match := usbPassed.FindStringSubmatch(line)
		if match != nil {
			if match[1] == "PASSED" {
				return Healthy
			}
			return Failing
		}
	}

	return NotApplicable
}

// RunSmartInfo runs the smartctl command to get health status
func RunSmartInfo(ctx context.Context, device string) ([]byte, error) {
	command := "smartctl -x " + device
	out, err := testexec.CommandContext(ctx, "sh", "-c", command).Output(testexec.DumpLogOnError)
	if err != nil {
		return out, err
	}
	return out, nil
}

// ParseUsbLife parses the output of smartctl
func ParseUsbLife(ctx context.Context, outLines []string) (storage.LifeStatus, error) {
	usbLife := regexp.MustCompile(`\s*231\s+SSD_Life_Left` + // ID and attribute name
		`\s+[P-][O-][S-][R-][C-][K-]` + // Flags
		`(\s+[0-9]{3}){3}` + // Three 1 to 3-digit numbers
		`\s+(NOW|-)` + // Fail indicator
		`\s+(?P<value>[1-9][0-9]*)`) // Non-zero raw value

	for _, line := range outLines {
		match := usbLife.FindStringSubmatch(line)
		if match != nil {
			lifeLeft, err := strconv.ParseInt(match[3], 10, 32)
			if err != nil {
				break
			}
			testing.ContextLog(ctx, "Life Left: ", lifeLeft)
			if lifeLeft > 5 {
				return Healthy, nil
			}
			return Failing, errors.Errorf("SSD Life Left: %d", lifeLeft)
		}
	}

	return NotApplicable, nil
}

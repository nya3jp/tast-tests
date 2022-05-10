// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// This will be updated to be read from system (s)
const (
	QuickDimTime         = 6 * time.Second
	QuickLockTime        = 126 * time.Second
	QuickDimDisableTime  = 420 * time.Second
	QuickLockDisableTime = 90 * time.Second
	PresentQuickDimTime  = 840 * time.Second
	PresentQuickLockTime = 90 * time.Second
)

// GetBrightness gets the current brightness of the dut
func GetBrightness(ctx context.Context, conn *ssh.Conn) (float64, error) {
	output, err := conn.CommandContext(ctx, "dbus-send", "--system",
		"--print-reply", "--type=method_call", "--dest=org.chromium.PowerManager", "/org/chromium/PowerManager",
		"org.chromium.PowerManager.GetScreenBrightnessPercent").Output()
	if err != nil {
		testing.ContextLog(ctx, "Getting brightness failed")
		return -1, err
	}

	mregex := regexp.MustCompile(`(.+)double ([0-9]+)`)
	result := mregex.FindStringSubmatch(strings.ToLower(string(output)))
	if len(result) < 2 {
		return -1, errors.New("no brightness found")
	}

	value, err := strconv.ParseFloat(result[2], 64)
	if err != nil {
		return -1, errors.Wrapf(err, "Conversion failed: %q", result[1])
	}
	return value, nil
}

// PollForDim is to see if the screen will dim during a designated amount of time
func PollForDim(ctx context.Context, brightness float64, timeout time.Duration, conn *ssh.Conn) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		autodimBrightness, err := GetBrightness(ctx, conn)
		if err != nil {
			return err
		}
		if autodimBrightness >= brightness {
			return errors.Errorf("Auto dim failed. Before human presence: %f, After human presence: %f", brightness, autodimBrightness)
		}
		if autodimBrightness == 0 {
			return errors.New("Screen is completely dark")
		}

		if autodimBrightness < brightness && autodimBrightness != 0 {
			testing.ContextLog(ctx, "Brightness changed to: ", autodimBrightness)
			return nil
		}
		return errors.New("Brightness not changed")
	}, &testing.PollOptions{
		Interval: 100 * time.Millisecond,
		Timeout:  timeout + 3*time.Second,
	}); err != nil {
		return errors.Wrap(err, "unexpected brightness change")
	}
	return nil
}

// WaitWithDelay return a 3s duration object
func WaitWithDelay(ctx context.Context, timeLength time.Duration) {
	testing.Sleep(ctx, 3*time.Second+timeLength)
}

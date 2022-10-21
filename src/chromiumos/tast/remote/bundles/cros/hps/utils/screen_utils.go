// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/hps"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

const (
	// BrightnessChangeTimeoutSlackDuration limits by how much the actual duration could differ from the expected duration.
	//
	// This discrepancy could be caused by HPS adapting to the exposure change.
	BrightnessChangeTimeoutSlackDuration = 5 * time.Second
	// LockOnLeave is the Settings name for LoL.
	LockOnLeave = "Lock-on-leave"
	// SecondPersonAlert is the Settings name for SPA.
	SecondPersonAlert = "Viewing protection (Beta)"
)

// GetBrightness gets the current brightness of the dut
func GetBrightness(ctx context.Context, conn *ssh.Conn) (float64, error) {
	output, err := conn.CommandContext(ctx, "dbus-send", "--system",
		"--print-reply", "--type=method_call", "--dest=org.chromium.PowerManager", "/org/chromium/PowerManager",
		"org.chromium.PowerManager.GetScreenBrightnessPercent").Output()
	if err != nil {
		return -1, errors.Wrap(err, "getting brightness failed")
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

// PollForBrightnessChange will poll until screen brightness differs from |initialBrightness| or the |timeout| occurs, whichever comes first.
func PollForBrightnessChange(ctx context.Context, initialBrightness float64, timeout time.Duration, conn *ssh.Conn) error {
	// This could take very long time, depending on the settings, at least a couple of minutes.
	testing.ContextLog(ctx, "Polling for brightness change: ", timeout.Seconds(), "s + ", BrightnessChangeTimeoutSlackDuration.Seconds(), "s (slack)")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		currentBrightness, err := GetBrightness(ctx, conn)

		if err != nil {
			return err
		}
		if currentBrightness != initialBrightness {
			return nil
		}
		return errors.New("Brightness not changed")
	}, &testing.PollOptions{
		Interval: 1 * time.Second,
		Timeout:  timeout + BrightnessChangeTimeoutSlackDuration,
	}); err != nil {
		return errors.Wrap(err, "error during polling")
	}
	return nil
}

func pollForDimHelper(initialBrightness, currentBrightness float64, checkForDark bool) error {
	if currentBrightness >= initialBrightness {
		return errors.Errorf("Auto dim failed. Before human presence: %f, After human presence: %f", initialBrightness, currentBrightness)
	}
	if currentBrightness == 0 {
		if !checkForDark {
			return errors.New("Screen went dark unexpectedly")
		}
		return nil
	}

	if currentBrightness < initialBrightness && currentBrightness != 0 && !checkForDark {
		return nil
	}
	return errors.New("Brightness not changed")
}

// PollForDim is to see if the screen will dim during a designated amount of time.
// Will poll for slightly longer than specified to allow for some slack.
func PollForDim(ctx context.Context, initialBrightness float64, timeout time.Duration, checkForDark bool, conn *ssh.Conn) error {
	// This could take very long time, depending on the settings, at least a couple of minutes.
	testing.ContextLog(ctx, "Polling for quick dim: ", timeout.Seconds(), "s + ", BrightnessChangeTimeoutSlackDuration.Seconds(), "s (slack)")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		currentBrightness, err := GetBrightness(ctx, conn)
		if err != nil {
			return err
		}
		return pollForDimHelper(initialBrightness, currentBrightness, checkForDark)
	}, &testing.PollOptions{
		Interval: 1 * time.Second,
		Timeout:  timeout + BrightnessChangeTimeoutSlackDuration,
	}); err != nil {
		return errors.Wrap(err, "error during polling")
	}
	currentBrightness, err := GetBrightness(ctx, conn)
	if err != nil {
		return err
	}
	testing.ContextLog(ctx, "brightness: ", currentBrightness)
	return nil
}

// WaitWithDelay waits for specified duration + some slack.
func WaitWithDelay(ctx context.Context, timeLength time.Duration) {
	testing.ContextLog(ctx, "Waiting for: ", timeLength.Seconds(), "s + 3s (slack)")
	testing.Sleep(ctx, 3*time.Second+timeLength)
}

// RetrieveHpsSenseSignal returns true if powerd currently sees positive HPS presence.
func RetrieveHpsSenseSignal(ctx context.Context, client pb.HpsServiceClient) (bool, error) {
	result, err := client.RetrieveHpsSenseSignal(ctx, &empty.Empty{})
	if err != nil {
		return false, err
	}
	if result.RawValue == "POSITIVE" {
		return true, nil
	}
	if result.RawValue == "NEGATIVE" {
		return false, nil
	}
	return false, errors.Errorf("unknown HPS Sense Signal: %q", result.RawValue)
}

// EnsureHpsSenseSignal will report an error if the current positivity of HPS signal doesn't patch |expectedSignal|.
func EnsureHpsSenseSignal(ctx context.Context, client pb.HpsServiceClient, expectedSignal bool) error {
	result, err := RetrieveHpsSenseSignal(ctx, client)
	if err != nil {
		return err
	}
	if result != expectedSignal {
		return errors.Errorf("HPS Sense Signal (%t) doesn't match the expectation (%t)", result, expectedSignal)
	}
	return nil
}

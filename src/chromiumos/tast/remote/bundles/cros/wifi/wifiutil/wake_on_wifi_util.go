// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/wifi"
)

// VerifyWakeOnWifiReason puts the DUT into suspend with timeout=duration,
// and calls the triggerFunc to wake the DUT, and checks the LastWakeReason.
func VerifyWakeOnWifiReason(
	ctx context.Context,
	tf *wificell.TestFixture,
	dut *dut.DUT,
	duration time.Duration,
	reason string,
	triggerFunc func(context.Context) error,
) error {
	// Some buffer time constants
	const (
		// Time buffer for the suspend gRPC/command to take effect.
		commandBufferTime = 10 * time.Second
		// Time buffer for the resume processing.
		resumeBufferTime = 15 * time.Second
		// Time buffer for the trigger to run.
		triggerBufferTime = 30 * time.Second
		// Time buffer for the RTC wake so the trigger can have time
		// to propagate to DUT.
		rtcBufferTime = 30 * time.Second

		suspendBuffer = resumeBufferTime + triggerBufferTime + rtcBufferTime
		contextBuffer = commandBufferTime + suspendBuffer
	)

	// Suspend the DUT in background.
	suspendErrCh := make(chan error, 1)
	// Wait for bg routine to end.
	defer func() { <-suspendErrCh }()
	suspendCtx, cancel := context.WithTimeout(ctx, duration+contextBuffer)
	defer cancel()
	go func() {
		defer close(suspendErrCh)
		_, err := tf.Suspend(suspendCtx, duration+suspendBuffer, false)
		suspendErrCh <- err
	}()
	// Wait for DUT to become unreachable.
	if err := dut.WaitUnreachable(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for DUT unreachable")
	}

	// Now DUT is suspended. Trigger the wake up source.
	triggerCtx, cancel := context.WithTimeout(ctx, triggerBufferTime)
	defer cancel()
	if err := triggerFunc(triggerCtx); err != nil {
		return errors.Wrap(err, "failed to trigger wake source")
	}

	// Wait for the suspend to end, i.e. DUT wakeup, and
	// see if we get any error.
	if err := <-suspendErrCh; err != nil {
		return errors.Wrap(err, "background suspend failed")
	}

	// DUT woke up successfully, check reason.
	req := &wifi.CheckLastWakeReasonRequest{Reason: reason}
	if _, err := tf.WifiClient().CheckLastWakeReason(ctx, req); err != nil {
		return err
	}
	return nil
}

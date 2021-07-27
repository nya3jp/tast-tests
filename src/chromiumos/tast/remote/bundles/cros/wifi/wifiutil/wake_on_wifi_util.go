// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"context"
	"time"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell"
	"chromiumos/tast/services/cros/wifi"
	"chromiumos/tast/testing"
)

// VerifyWakeOnWifiReason puts the DUT into suspend with timeout=duration,
// calls the triggerFunc to wake the DUT, and checks the LastWakeReason.
func VerifyWakeOnWifiReason(
	ctx context.Context,
	tf *wificell.TestFixture,
	dut *dut.DUT,
	duration time.Duration,
	reason string,
	triggerFunc func(context.Context) error,
) error {
	// Some buffer time constants.
	const (
		// Time buffer for the suspend gRPC/command to take effect.
		commandBufferTime = 10 * time.Second
		// Time buffer for the trigger to run.
		triggerBufferTime = 30 * time.Second
		// Time buffer for the RTC wake so we can have some more time
		// for the trigger to propagate to DUT and the DUT to resume.
		rtcBufferTime = 45 * time.Second

		suspendBuffer = triggerBufferTime + rtcBufferTime
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
		// Use the raw gRPC to have better control.
		_, err := tf.WifiClient().ShillServiceClient.Suspend(suspendCtx, &wifi.SuspendRequest{
			WakeUpTimeout:  (duration + suspendBuffer).Nanoseconds(),
			CheckEarlyWake: false,
		})
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

// DUTActive accesses ex_system_powerstate to tell if the DUT is in active (S0) state.
func DUTActive(ctx context.Context, servoInst *servo.Servo) (bool, error) {
	state, err := servoInst.GetECSystemPowerState(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get ec_system_power_state")
	}
	testing.ContextLog(ctx, "state: ", state)
	if state == "S0" {
		return true, nil
	}
	return false, nil
}

// WaitDUTActive uses servo to wait for DUT to reach/leave active (S0) state.
func WaitDUTActive(ctx context.Context, servoInst *servo.Servo, expectActive bool, timeout time.Duration) error {
	var expectedDesc string
	if expectActive {
		expectedDesc = "active"
	} else {
		expectedDesc = "inactive"
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		active, err := DUTActive(ctx, servoInst)
		if err != nil {
			return testing.PollBreak(err)
		}
		if active != expectActive {
			return errors.Errorf("DUT is not yet %s", expectedDesc)
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  timeout,
		Interval: time.Second,
	}); err != nil {
		return errors.Wrapf(err, "failed to wait for DUT to become %s", expectedDesc)
	}
	return nil
}

// DarkResumeSuspend suspends the DUT with powerd_dbus_suspend with dark resume
// enabled. On successful call, a shortened context and a cleanup function to
// wake up the DUT are returned.
func DarkResumeSuspend(fullCtx context.Context, d *dut.DUT, servoInst *servo.Servo) (context.Context, func() error, error) {
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)

	done := make(chan error, 1)
	go func(ctx context.Context) {
		defer close(done)
		out, err := d.Command("powerd_dbus_suspend", "--disable_dark_resume=false", "--print_wakeup_type=true").CombinedOutput(ctx)
		testing.ContextLog(ctx, "DEBUG: powerd_dbus_suspend output: ", string(out))
		done <- err
	}(ctx)

	if err := WaitDUTActive(ctx, servoInst, false, 10*time.Second); err != nil {
		cancel()
		return ctx, nil, errors.Wrap(err, "failed to wait for DUT to be suspended")
	}
	cleanupFunc := func() error {
		// Cancel the context used by the goroutine before leaving.
		defer cancel()

		active, err := DUTActive(fullCtx, servoInst)
		if err != nil {
			return err
		}
		if !active {
			testing.ContextLog(ctx, "Try to wake up DUT with power_key:press")
			if err := servoInst.KeypressWithDuration(fullCtx, servo.PowerKey, servo.DurPress); err != nil {
				return errors.Wrap(err, "failed to trigger power key press with servo")
			}
			if err := WaitDUTActive(ctx, servoInst, true, 10*time.Second); err != nil {
				return errors.New("failed to wake up DUT with servo")
			}
			testing.ContextLog(ctx, "DUT back to active")
		}
		if err := <-done; err != nil {
			return errors.Wrap(err, "powerd_dbus_suspend failed")
		}
		return nil
	}
	return ctx, cleanupFunc, nil
}

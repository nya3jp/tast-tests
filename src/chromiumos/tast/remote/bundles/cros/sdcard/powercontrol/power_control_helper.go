// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package powercontrol

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/security"
	"chromiumos/tast/testing"
)

// ChromeOSLogin performs login to DUT.
func ChromeOSLogin(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) error {
	cl, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(ctx)
	client := security.NewBootLockboxServiceClient(cl.Conn)
	if _, err := client.NewChromeLogin(ctx, &empty.Empty{}); err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	return nil
}

// ValidatePrevSleepState sleep state from cbmem command output.
func ValidatePrevSleepState(ctx context.Context, dut *dut.DUT, sleepStateValue int) error {
	// Command to check previous sleep state.
	const cmd = "cbmem -c | grep 'prev_sleep_state' | tail -1"
	out, err := dut.Conn().CommandContext(ctx, "sh", "-c", cmd).Output()
	if err != nil {
		return errors.Wrapf(err, "failed to execute %q command", cmd)
	}

	got := strings.TrimSpace(string(out))
	want := fmt.Sprintf("prev_sleep_state %d", sleepStateValue)

	if got != want {
		return errors.Errorf("unexpected sleep state = got %q, want %q", got, want)
	}
	return nil
}

// ShutdownAndWaitForPowerState verifies powerState(S5 or G3) after shutdown.
func ShutdownAndWaitForPowerState(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT, powerState string) error {
	powerOffCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := dut.Conn().CommandContext(powerOffCtx, "shutdown", "-h", "now").Run(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return errors.Wrap(err, "failed to execute shutdown command")
	}
	sdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := dut.WaitUnreachable(sdCtx); err != nil {
		return errors.Wrap(err, "failed to wait for unreachable")
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		got, err := pxy.Servo().GetECSystemPowerState(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get EC power state")
		}
		if want := powerState; got != want {
			return errors.Errorf("unexpected DUT EC power state = got %q, want %q", got, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 20 * time.Second})
}

// PowerOntoDUT performs power normal press to wake DUT.
func PowerOntoDUT(ctx context.Context, pxy *servo.Proxy, dut *dut.DUT) error {
	waitCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	if err := pxy.Servo().KeypressWithDuration(ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Wrap(err, "failed to power button press")
	}
	if err := dut.WaitConnect(waitCtx); err != nil {
		return errors.Wrap(err, "failed to wait connect DUT")
	}
	return nil
}

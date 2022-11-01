// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/pkcs11"
	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	hwsecremote "chromiumos/tast/remote/hwsec"
	"chromiumos/tast/testing"
)

// PowerButtonHelper is a helper interface that can press the power button
// on the device.
type PowerButtonHelper interface {
	PressAndRelease(ctx context.Context) error
}

// ServoPowerButtonHelper presses the power button using servo key press.
// Note that this will only work on specific test suites where servo-micro
// is connected (e.g., firmware_cr50).
type ServoPowerButtonHelper struct {
	svo *servo.Servo
}

// NewServoPowerButtonHelper creates a new ServoPowerButtonHelper.
func NewServoPowerButtonHelper(svo *servo.Servo) ServoPowerButtonHelper {
	return ServoPowerButtonHelper{svo}
}

// PressAndRelease implements PowerButtonHelper.PressAndRelease.
func (helper ServoPowerButtonHelper) PressAndRelease(ctx context.Context) error {
	return helper.svo.KeypressWithDuration(ctx, servo.PowerKey, servo.DurTab)
}

// SocketPowerButtonHelper presses the power button by sending bytes to
// the GPIO power button socket. Note that this will only work on VMs
// running ti50-emulator (dependencies "tpm2-simulator" + "gsc").
type SocketPowerButtonHelper struct {
	cmd hwsec.CmdRunner
}

// NewSocketPowerButtonHelper creates a new SocketPowerButtonHelper.
func NewSocketPowerButtonHelper(cmd hwsec.CmdRunner) SocketPowerButtonHelper {
	return SocketPowerButtonHelper{cmd}
}

// PressAndRelease implements PowerButtonHelper.PressAndRelease.
func (helper SocketPowerButtonHelper) PressAndRelease(ctx context.Context) error {
	const (
		socketCommandTempl string = "echo -e %s | socat -t1 unix-connect:/run/tpm2-simulator/sockets/gpioPwrBtn -"
		zero               string = "0"
		one                string = "1"
	)

	// Sending the character zero to the socket triggers a power button pressed
	// signal, while sending the character one triggers a power button released
	// signal.
	if _, err := helper.cmd.Run(ctx, "sh", "-c", fmt.Sprintf(socketCommandTempl, one)); err != nil {
		return errors.Wrap(err, "failed to press power button")
	}
	testing.Sleep(ctx, 500*time.Millisecond)
	if _, err := helper.cmd.Run(ctx, "sh", "-c", fmt.Sprintf(socketCommandTempl, zero)); err != nil {
		return errors.Wrap(err, "failed to release power button")
	}
	return nil
}

// SetU2fdFlags sets the flags and restarts u2fd, which will re-create the u2f device.
func SetU2fdFlags(ctx context.Context, helper *hwsecremote.FullHelperRemote, u2f, g2f, userKeys bool) (retErr error) {
	const (
		uf2ForcePath      = "/var/lib/u2f/force/u2f.force"
		gf2ForcePath      = "/var/lib/u2f/force/g2f.force"
		userKeysForcePath = "/var/lib/u2f/force/user_keys.force"
	)

	cmd := helper.CmdRunner()
	dCtl := helper.DaemonController()

	if err := dCtl.Stop(ctx, hwsec.U2fdDaemon); err != nil {
		return errors.Wrap(err, "failed to stop u2fd")
	}
	defer func(ctx context.Context) {
		if err := dCtl.Start(ctx, hwsec.U2fdDaemon); err != nil {
			if retErr != nil {
				testing.ContextLog(ctx, "Failed to restart u2fd: ", err)
			} else {
				retErr = errors.Wrap(err, "failed to restart u2fd")
			}
		}
	}(ctx)

	// Remove flags.
	if _, err := cmd.Run(ctx, "sh", "-c", "rm -f /var/lib/u2f/force/*.force"); err != nil {
		return errors.Wrap(err, "failed to remove flags")
	}
	if u2f {
		if _, err := cmd.Run(ctx, "touch", uf2ForcePath); err != nil {
			return errors.Wrap(err, "failed to set u2f flag")
		}
	}
	if g2f {
		if _, err := cmd.Run(ctx, "touch", gf2ForcePath); err != nil {
			return errors.Wrap(err, "failed to set g2f flag")
		}
	}
	if userKeys {
		if _, err := cmd.Run(ctx, "touch", userKeysForcePath); err != nil {
			return errors.Wrap(err, "failed to set userKeys flag")
		}
	}
	return nil
}

// EnsureChapsSlotsInitialized ensures chaps is initialized.
func EnsureChapsSlotsInitialized(ctx context.Context, chaps *pkcs11.Chaps) error {
	return testing.Poll(ctx, func(context.Context) error {
		slots, err := chaps.ListSlots(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to list chaps slots")
		}
		testing.ContextLog(ctx, slots)
		if len(slots) < 2 {
			return errors.Wrap(err, "chaps initialization hasn't finished")
		}
		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
		Interval: time.Second,
	})
}

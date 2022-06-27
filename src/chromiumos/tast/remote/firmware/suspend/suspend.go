// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to suspend/wake the DUT.

package suspend

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
)

const (
	// Default timeout while waiting for DUT to reconnect after wake
	wakeTimeout = 20 * time.Second
	// Default interval to check for DUT reconnection after wake
	wakeInterval = time.Second
	// Default delay for suspend in seconds
	suspendDelaySeconds = 3
	// Default location to mount power manager control directory
	tmpPowerManagerPath = "/tmp/power_manager"
)

// State represents a given suspend state
type State string

const (
	// StateS3 for S3
	StateS3 State = "S3"
	// StateS0ix for S0ix
	StateS0ix State = "S0ix"
)

// SuspendArgs are arguments for SuspendDUT
type SuspendArgs struct {
	delay int // Delay before suspending in seconds
}

// DefaultSuspendArgs creates default arguments for SuspendDUT
func DefaultSuspendArgs() SuspendArgs {
	return SuspendArgs{
		delay: suspendDelaySeconds,
	}
}

// WakeArgs are arguments for WakeDUT
type WakeArgs struct {
	timeout  time.Duration // Duration to wait for DUT to wakeup/reconnect
	interval time.Duration // How often to check for DUT wakeup/reconnect
}

// DefaultWakeArgs creates default arguments for WakeDUT
func DefaultWakeArgs() WakeArgs {
	return WakeArgs{
		timeout:  wakeTimeout,
		interval: wakeInterval,
	}
}

// Context is used to manage suspending and resuming a DUT
type Context struct {
	h   *firmware.Helper
	ctx context.Context
}

// NewContext creates a suspend context from a normal context and firmware helper.
// This also does some setup to make any power_manager changes last until reboot.
func NewContext(ctx context.Context, h *firmware.Helper) (*Context, error) {
	err := h.DUT.Conn().CommandContext(ctx, "sh", "-c", fmt.Sprintf("mkdir -p %s && "+
		"echo 0 > %s/suspend_to_idle && "+
		"mount --bind %s /var/lib/power_manager && "+
		"restart powerd",
		tmpPowerManagerPath, tmpPowerManagerPath, tmpPowerManagerPath)).Run()

	if err != nil {
		return nil, err
	}

	s := Context{
		h:   h,
		ctx: ctx,
	}
	return &s, nil
}

// SuspendDUT attempts to suspend a DUT to the given state.
func (s *Context) SuspendDUT(state State, args SuspendArgs) error {
	if err := s.setSuspendToIdle(state == StateS0ix); err != nil {
		return err
	}

	cmd := s.h.DUT.Conn().CommandContext(s.ctx, "powerd_dbus_suspend", fmt.Sprintf("--delay=%d", args.delay))
	if err := cmd.Start(); err != nil {
		return errors.Errorf("failed to invoke powerd_dbus_suspend: %s", err)
	}

	testing.Sleep(s.ctx, time.Second)
	if err := s.h.WaitForPowerStates(s.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, string(state)); err != nil {
		return errors.Errorf("failed to get power state %s: %s", state, err)
	}

	return nil
}

// WakeDUT attempts to wake the DUT by simulating a power button press.
func (s *Context) WakeDUT(args WakeArgs) error {
	if err := s.h.Servo.KeypressWithDuration(s.ctx, servo.PowerKey, servo.DurPress); err != nil {
		return errors.Errorf("failed to press power key on DUT: %s", err)
	}

	if err := s.h.WaitForPowerStates(s.ctx, firmware.PowerStateInterval, firmware.PowerStateTimeout, "S0"); err != nil {
		return errors.Errorf("DUT failed to reach S0 after power button pressed: %s", err)
	}

	err := testing.Poll(s.ctx, func(ctx context.Context) error {
		if !s.h.DUT.Connected(ctx) {
			return errors.New("waiting for DUT to reconnect")
		}

		return nil

	}, &testing.PollOptions{Timeout: args.timeout, Interval: args.interval})

	if err != nil {
		return errors.New("failed to reconnect to DUT after entering S0")
	}

	return nil
}

// VerifySupendWake determines if the DUT supports a given state.
// The files in `/sys/power` may not always be accurate so this function
// actually suspends the DUT to verify that everything works.
func (s *Context) VerifySupendWake(state State) error {
	if kernelSupported, err := s.isSupportedKernel(state); err != nil || !kernelSupported {
		return errors.New("state not supported by kernel")
	}

	suspendArgs := DefaultSuspendArgs()
	if err := s.SuspendDUT(state, suspendArgs); err != nil {
		return err
	}

	wakeArgs := DefaultWakeArgs()
	if err := s.WakeDUT(wakeArgs); err != nil {
		return err
	}

	return nil
}

// GetKernelSuspendCount returns the number of successful suspends registered by the kernel.
func (s *Context) GetKernelSuspendCount() (int, error) {
	resultBytes, err := s.h.DUT.Conn().CommandContext(s.ctx, "cat", "/sys/kernel/debug/suspend_stats").Output()
	if err != nil {
		return -1, err
	}

	lines := strings.Split(string(resultBytes), "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "success") {
			continue
		}

		split := strings.Split(line, ":")
		if len(split) != 2 {
			return -1, errors.New("improperly formatted success line")
		}

		return strconv.Atoi(strings.TrimSpace(split[1]))
	}

	return -1, errors.New("failed to find success line")
}

// Close the Context and perform cleanup.
func (s *Context) Close() error {
	return s.h.DUT.Conn().CommandContext(s.ctx, "sh", "-c", "umount /var/lib/power_manager && restart powerd").Run()
}

// setSuspendToIdle sets the suspend_to_idle value which controls if powerd will suspend to S0ix
func (s *Context) setSuspendToIdle(value bool) error {
	idleValue := "0"
	if value {
		idleValue = "1"
	}

	return s.h.DUT.Conn().CommandContext(s.ctx, "sh", "-c", fmt.Sprintf("echo %s > %s/suspend_to_idle",
		idleValue, tmpPowerManagerPath)).Run()
}

// runWithExitStatus is a utility function to run a command and return its status code.
func (s *Context) runWithExitStatus(name string, args ...string) (int, error) {
	err := s.h.DUT.Conn().CommandContext(s.ctx, name, args...).Run()
	if err == nil {
		// No error so we the command executed with exit code 0.
		return 0, nil
	}

	if exitError := err.(*ssh.ExitError); exitError != nil {
		return exitError.ExitStatus(), nil
	}

	return -1, err
}

// isSupportedKernel reports if the kernel believes it can enter the given state.
func (s *Context) isSupportedKernel(state State) (bool, error) {
	path := ""
	name := ""

	if state == StateS0ix {
		path = "/sys/power/state"
		name = "freeze"
	} else if state == StateS3 {
		path = "/sys/power/mem_sleep"
		name = "deep"
	} else {
		return false, errors.Errorf("unsupported suspend state %s", state)
	}

	if ret, err := s.runWithExitStatus("grep", "-q", name, path); err != nil || ret != 0 {
		return false, err
	}

	return true, nil
}

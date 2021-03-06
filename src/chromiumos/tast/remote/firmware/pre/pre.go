// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre implements preconditions for firmware tests.
package pre

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/checkers"
	pb "chromiumos/tast/services/cros/firmware"
	"chromiumos/tast/testing"
)

// Value contains fields that are useful for tests.
type Value struct {
	BootMode common.BootMode
	GBBFlags pb.GBBFlagsState
	Helper   *firmware.Helper
}

// impl contains fields that are useful for Precondition methods.
type impl struct {
	v              *Value
	origBootMode   *common.BootMode
	origGBBFlags   *pb.GBBFlagsState
	timeout        time.Duration
	usbKeyVerified bool
}

// NormalMode boots to Normal Mode.
func NormalMode() testing.Precondition {
	return normalMode
}

// DevMode boots to Developer Mode via the keypress workflow.
func DevMode() testing.Precondition {
	return devMode
}

// DevModeGBB boots to Developer Mode via GBB force.
// This is stabler than the keypress workflow, but is not appropriate for all tests.
func DevModeGBB() testing.Precondition {
	return devModeGBB
}

// RecMode boots to Recover Mode. Tests which use RecMode() need to use the Attr `firmware_usb` also.
func RecMode() testing.Precondition {
	return recMode
}

// newPrecondition creates an instance of firmware Precondition.
func newPrecondition(mode common.BootMode, forceDev bool) testing.Precondition {
	flags := pb.GBBFlagsState{Clear: common.AllGBBFlags(), Set: common.FAFTGBBFlags()}
	if forceDev {
		flags.Set = append(flags.Set, pb.GBBFlag_FORCE_DEV_SWITCH_ON)
	}
	return &impl{
		v: &Value{
			BootMode: mode,
			// Default GBBFlagsState for firmware testing.
			GBBFlags: flags,
		},
		// The maximum time that the Prepare method should take, adjust as needed.
		timeout: 5 * time.Minute,
	}
}

// Create the preconditions to be shared by tests in the run.
var (
	normalMode = newPrecondition(common.BootModeNormal, false)
	devMode    = newPrecondition(common.BootModeDev, false)
	devModeGBB = newPrecondition(common.BootModeDev, true)
	recMode    = newPrecondition(common.BootModeRecovery, false)
)

// Prepare ensures that the DUT is booted into the specified mode.
func (i *impl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	// Initialize Helper during the first Prepare invocation or after a test has niled it.
	if i.v.Helper == nil {
		s.Log("Creating a new firmware Helper instance for Precondition: ", i.String())
		i.initHelper(ctx, s)
	} else {
		// Close the servo to reset pd role, watchdogs, etc.
		i.v.Helper.CloseServo(ctx)
		// Close the RPC client in case the DUT rebooted at some point, and it doesn't recover well.
		i.v.Helper.CloseRPCConnection(ctx)
	}

	if err := i.v.Helper.RequireServo(ctx); err != nil {
		s.Fatal("Could not connect to servod: ", err)
	}
	if err := i.v.Helper.EnsureDUTBooted(ctx); err != nil {
		s.Fatal("DUT is offline before test start: ", err)
	}

	// The GBB flags might prevent booting into the correct mode, so check the boot mode,
	// then save the GBB flags, then set the GBB flags, and finally reboot into the right mode.
	mode, err := i.v.Helper.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Fatal("Could not get current boot mode: ", err)
	}

	// If this is the first Prepare invocation, save the starting boot mode.
	if i.origBootMode == nil {
		testing.ContextLogf(ctx, "Saving boot mode %q for restoration upon completion of all tests under this precondition", mode)
		i.origBootMode = &mode
	}

	if err := i.v.Helper.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Failed to require BiosServiceClient: ", err)
	}

	testing.ContextLog(ctx, "Get current GBB flags")
	curr, err := i.v.Helper.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Getting current GBB Flags failed: ", err)
	}

	// If this is the first Prepare invocation, save the starting GBB flags.
	if i.origGBBFlags == nil {
		testing.ContextLogf(ctx, "Saving GBB flags %+v for restoration upon completion of all tests under this precondition", curr.Set)
		i.origGBBFlags = curr
	}

	rebootRequired := false
	if common.GBBFlagsStatesEqual(i.v.GBBFlags, *curr) {
		testing.ContextLog(ctx, "GBBFlags are already proper")
	} else {
		if err := i.setAndCheckGBBFlags(ctx, i.v.GBBFlags); err != nil {
			s.Fatal("SetAndCheckGBBFlags failed: ", err)
		}
		if common.GBBFlagsChanged(*curr, i.v.GBBFlags, common.RebootRequiredGBBFlags()) {
			testing.ContextLog(ctx, "Resetting DUT due to GBB flag change")
			rebootRequired = true
		}
	}

	if mode != i.v.BootMode {
		testing.ContextLogf(ctx, "Current boot mode is %q, rebooting to %q to satisfy precondition", mode, i.v.BootMode)
		rebootRequired = true
	}

	if rebootRequired {
		// If rebooting to recovery mode, verify the usb key, but only once, because it's slow and unlikely to break in the middle of tests.
		if i.v.BootMode == common.BootModeRecovery && !i.usbKeyVerified {
			if err := i.v.Helper.SetupUSBKey(ctx, s.CloudStorage()); err != nil {
				s.Fatal("USBKey not working: ", err)
			}
			i.usbKeyVerified = true
		}
		opts := []firmware.ModeSwitchOption{firmware.AssumeGBBFlagsCorrect}
		if i.v.ForcesDevMode() {
			opts = append(opts, firmware.AllowGBBForce)
		}
		if err := i.rebootToMode(ctx, i.v.BootMode, opts...); err != nil {
			s.Fatalf("Failed to reboot to mode %q: %s", i.v.BootMode, err)
		}
	}

	return i.v
}

// Close restores the boot mode and GBB flags that were stored before the first Prepare().
func (i *impl) Close(ctx context.Context, s *testing.PreState) {
	defer func() {
		i.destroyHelper(ctx, s)
		i.origBootMode = nil
		i.origGBBFlags = nil
	}()

	i.initHelper(ctx, s)
	// Close the servo to reset pd role, watchdogs, etc.
	i.v.Helper.CloseServo(ctx)
	// Close the RPC client in case the DUT rebooted at some point, and it doesn't recover well.
	i.v.Helper.CloseRPCConnection(ctx)

	if err := i.v.Helper.EnsureDUTBooted(ctx); err != nil {
		s.Fatal("DUT is offline after test end: ", err)
	}

	if err := i.restoreBootMode(ctx); err != nil {
		s.Error("Could not restore BootMode: ", err)
	}

	if err := i.restoreGBBFlags(ctx); err != nil {
		s.Error("Could not restore GBB flags: ", err)
	}
}

// String identifies this Precondition.
func (i *impl) String() string {
	name := string(i.v.BootMode)
	if i.v.ForcesDevMode() {
		name += "-gbb"
	}
	return name
}

// Timeout is the max time needed to prepare this Precondition.
func (i *impl) Timeout() time.Duration {
	return i.timeout
}

// initHelper ensures that the impl has a working Helper instance.
func (i *impl) initHelper(ctx context.Context, s *testing.PreState) {
	if i.v.Helper == nil {
		// Make sure the test included the right software deps
		crossystem := false
		flashrom := false
		for _, v := range s.SoftwareDeps() {
			if v == "crossystem" {
				crossystem = true
			} else if v == "flashrom" {
				flashrom = true
			}
		}
		if !crossystem {
			s.Fatal("SoftwareDeps does not include 'crossystem'")
		}
		if !flashrom {
			s.Fatal("SoftwareDeps does not include 'flashrom'")
		}
		servoSpec, _ := s.Var("servo")
		i.v.Helper = firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), servoSpec)
	}
}

// destroyHelper closes and nils any existing Helper instance.
func (i *impl) destroyHelper(ctx context.Context, s *testing.PreState) {
	if i.v.Helper == nil {
		return
	}
	if err := i.v.Helper.Close(ctx); err != nil {
		s.Log("Closing helper failed: ", err)
	}
	i.v.Helper = nil
}

// restoreBootMode restores DUT's boot mode.
func (i *impl) restoreBootMode(ctx context.Context) error {
	// Can't Restore the boot mode if unknown.
	if i.origBootMode == nil {
		return nil
	}

	mode, err := i.v.Helper.Reporter.CurrentBootMode(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get boot mode")
	}

	if mode != *i.origBootMode {
		testing.ContextLogf(ctx, "Restoring boot mode from %q to %q", mode, *i.origBootMode)
		if err := i.rebootToMode(ctx, *i.origBootMode); err != nil {
			return errors.Wrap(err, "failed to restore boot mode")
		}
	}

	return nil
}

// rebootToMode reboots to the specified mode using the ModeSwitcher, it assumes the helper is present.
func (i *impl) rebootToMode(ctx context.Context, mode common.BootMode, opts ...firmware.ModeSwitchOption) error {
	ms, err := firmware.NewModeSwitcher(ctx, i.v.Helper)
	if err != nil {
		return err
	}
	if err := ms.RebootToMode(ctx, mode, opts...); err != nil {
		return err
	}
	return nil
}

// restoreGBBFlags restores GBB Flags as it was prior to the precondition starting.
func (i *impl) restoreGBBFlags(ctx context.Context) error {
	if i.origGBBFlags == nil {
		return nil
	}

	if err := i.v.Helper.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to require BiosServiceClient")
	}

	testing.ContextLog(ctx, "Get current GBB flags")
	curr, err := i.v.Helper.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		return errors.Wrap(err, "getting current GBB Flags failed")
	}

	if common.GBBFlagsStatesEqual(*i.origGBBFlags, *curr) {
		return nil
	}

	if err := i.setAndCheckGBBFlags(ctx, *i.origGBBFlags); err != nil {
		return errors.Wrap(err, "restoreGBBFlags failed")
	}
	if err := i.rebootIfRequired(ctx, *curr, *i.origGBBFlags); err != nil {
		return errors.Wrap(err, "rebooting after required flags changed failed")
	}

	return nil
}

// setAndCheckGBBFlags sets and reads back the GBBFlags to ensure correctness.
func (i *impl) setAndCheckGBBFlags(ctx context.Context, req pb.GBBFlagsState) error {
	if err := i.v.Helper.RequireBiosServiceClient(ctx); err != nil {
		return err
	}

	if _, err := i.v.Helper.BiosServiceClient.ClearAndSetGBBFlags(ctx, &req); err != nil {
		return err
	}

	checker := checkers.New(i.v.Helper)
	if err := checker.GBBFlags(ctx, req); err != nil {
		return err
	}

	return nil
}

// rebootIfRequired reboots the DUT if any flags that require a reboot have changed.
func (i *impl) rebootIfRequired(ctx context.Context, a, b pb.GBBFlagsState) error {
	if !common.GBBFlagsChanged(a, b, common.RebootRequiredGBBFlags()) {
		return nil
	}
	ms, err := firmware.NewModeSwitcher(ctx, i.v.Helper)
	if err != nil {
		return err
	}
	testing.ContextLog(ctx, "Resetting DUT due to GBB flag change")
	return ms.ModeAwareReboot(ctx, firmware.WarmReset)
}

// ForcesDevMode reports whether the Precondition forces dev mode.
func (v *Value) ForcesDevMode() bool {
	for _, flag := range v.GBBFlags.Set {
		if flag == pb.GBBFlag_FORCE_DEV_SWITCH_ON {
			return true
		}
	}
	return false
}

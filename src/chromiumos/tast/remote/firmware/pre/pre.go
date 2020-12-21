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
	v            *Value
	origBootMode *common.BootMode
	origGBBFlags *pb.GBBFlagsState
	timeout      time.Duration
}

// NormalMode boots to Normal Mode.
func NormalMode() testing.Precondition {
	return normalMode
}

// DevMode boots to Developer Mode.
func DevMode() testing.Precondition {
	return devMode
}

// RecMode boots to Recover Mode.
func RecMode() testing.Precondition {
	return recMode
}

// newPrecondition creates an instance of firmware Precondition.
func newPrecondition(mode common.BootMode) testing.Precondition {
	return &impl{
		v: &Value{
			BootMode: mode,
			// Default GBBFlagsState for firmware testing.
			GBBFlags: pb.GBBFlagsState{Clear: common.AllGBBFlags(), Set: common.FAFTGBBFlags()},
		},
		// The maximum time that the Prepare method should take, adjust as needed.
		timeout: 5 * time.Minute,
	}
}

// Create the preconditions to be shared by tests in the run.
var (
	normalMode = newPrecondition(common.BootModeNormal)
	devMode    = newPrecondition(common.BootModeDev)
	recMode    = newPrecondition(common.BootModeRecovery)
)

// Prepare ensures that the DUT is booted into the specified mode.
func (i *impl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	// Initialize Helper during the first Prepare invocation or after a test has niled it.
	if i.v.Helper == nil {
		s.Log("Creating a new firmware Helper instance for Precondition: ", i.String())
		i.initHelper(ctx, s)
	}

	i.setupBootMode(ctx, s)
	i.setupGBBFlags(ctx, s)

	return i.v
}

// Close restores the boot mode before the first Prepare().
func (i *impl) Close(ctx context.Context, s *testing.PreState) {
	defer func() {
		i.destroyHelper(ctx, s)
		i.origBootMode = nil
	}()

	// Don't reuse the Helper, as the helper's servo RPC connection may be down.
	i.initHelper(ctx, s)

	i.restoreGBBFlags(ctx, s)
	i.restoreBootMode(ctx, s)
}

// String identifies this Precondition.
func (i *impl) String() string {
	return string(i.v.BootMode)
}

// Timeout is the max time needed to prepare this Precondition.
func (i *impl) Timeout() time.Duration {
	return i.timeout
}

// initHelper always creates a new Helper instance.
func (i *impl) initHelper(ctx context.Context, s *testing.PreState) {
	i.destroyHelper(ctx, s)
	i.v.Helper = firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), s.RequiredVar("servo"))
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

// setupBootMode the DUT to the correct mode if it's in a different one, saving the original one.
func (i *impl) setupBootMode(ctx context.Context, s *testing.PreState) {
	mode, err := i.v.Helper.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Fatal("Could not get current boot mode: ", err)
	}

	// This is the first Prepare invocation, save the starting boot mode.
	if i.origBootMode == nil {
		s.Logf("Saving boot mode %q for restoration upon completion of all tests under this precondition", mode)
		i.origBootMode = &mode
	}

	if mode != i.v.BootMode {
		s.Logf("Current boot mode is %q, rebooting to %q to satisfy precondition", mode, i.v.BootMode)
		if err := i.rebootToMode(ctx, i.v.BootMode); err != nil {
			s.Fatalf("Failed to reboot to mode %q: %v", i.v.BootMode, err)
		}
	}
}

// restoreBootMode restores DUT's boot mode.
func (i *impl) restoreBootMode(ctx context.Context, s *testing.PreState) {
	// Can't Restore the boot mode if unknown.
	if i.origBootMode == nil {
		return
	}

	mode, err := i.v.Helper.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Error("Could not get boot mode: ", err)
		return
	}

	if mode != *i.origBootMode {
		s.Logf("Restoring boot mode from %q to %q", mode, *i.origBootMode)
		if err := i.rebootToMode(ctx, *i.origBootMode); err != nil {
			s.Error("Failed to restore boot mode: ", err)
			return
		}
	}
}

// rebootToMode reboots to the specified mode using the ModeSwitcher, it assumes the helper is present.
func (i *impl) rebootToMode(ctx context.Context, mode common.BootMode) error {
	ms, err := firmware.NewModeSwitcher(ctx, i.v.Helper)
	if err != nil {
		return err
	}
	if err := ms.RebootToMode(ctx, mode); err != nil {
		return err
	}
	return nil
}

// setupGBBFlags sets and clears GBB Flags for firmware testing.
func (i *impl) setupGBBFlags(ctx context.Context, s *testing.PreState) {

	if err := i.v.Helper.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Failed to require BiosServiceClient: ", err)
	}

	orig, err := i.v.Helper.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Getting original GBB Flags failed: ", err)
	}
	i.origGBBFlags = orig

	if common.GBBFlagsStatesEqual(i.v.GBBFlags, *i.origGBBFlags) {
		s.Log("GBBFlags are already proper")
		return
	}

	if err := i.setAndCheckGBBFlags(ctx, i.v.GBBFlags); err != nil {
		s.Fatal("SetAndCheckGBBFlags failed: ", err)
	}

	if err := i.rebootIfRequired(ctx, *i.origGBBFlags, i.v.GBBFlags); err != nil {
		s.Fatal("Rebooting after required flags changed failed: ", err)
	}
}

// restoreGBBFlags restores GBB Flags as it was prior to setupGBBFlags.
func (i *impl) restoreGBBFlags(ctx context.Context, s *testing.PreState) {
	if i.origGBBFlags == nil {
		return
	}

	if err := i.v.Helper.RequireBiosServiceClient(ctx); err != nil {
		s.Error("Failed to require BiosServiceClient: ", err)
		return
	}

	curr, err := i.v.Helper.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Error("Getting current GBB Flags failed: ", err)
		return
	}

	if !common.GBBFlagsStatesEqual(*i.origGBBFlags, *curr) {
		if err := i.setAndCheckGBBFlags(ctx, *i.origGBBFlags); err != nil {
			s.Error("RestoreGBBFlags failed: ", err)
			return
		}
		if err := i.rebootIfRequired(ctx, *curr, *i.origGBBFlags); err != nil {
			s.Error("Rebooting after required flags changed failed: ", err)
			return
		}
	}
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

// rebootIfRequired reboots the DUT if any flags that require a reboot has changed.
func (i *impl) rebootIfRequired(ctx context.Context, a, b pb.GBBFlagsState) error {
	if common.GBBFlagsChanged(a, b, common.RebootRequiredGBBFlags()) {
		ms, err := firmware.NewModeSwitcher(ctx, i.v.Helper)
		if err != nil {
			return err
		}
		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
			return err
		}
	}
	return nil
}

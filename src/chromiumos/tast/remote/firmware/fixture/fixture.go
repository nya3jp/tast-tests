// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fixture implements fixtures for firmware tests.
package fixture

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

// Fixture names for the tests to use.
const (
	NormalMode string = "bootModeNormal"
	DevMode    string = "bootModeDev"
	DevModeGBB string = "bootModeDevGBB"
	RecMode    string = "bootModeRec"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            NormalMode,
		Desc:            "Reboot into normal mode before test",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeNormal, false),
		Vars:            []string{"servo"},
		SetUpTimeout:    5 * time.Minute,
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		ServiceDeps:     []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Data:            []string{firmware.ConfigFile},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            DevMode,
		Desc:            "Reboot into dev mode before test",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeDev, false),
		Vars:            []string{"servo"},
		SetUpTimeout:    5 * time.Minute,
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		ServiceDeps:     []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Data:            []string{firmware.ConfigFile},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            DevModeGBB,
		Desc:            "Reboot into dev mode using GBB flags before test",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeDev, true),
		Vars:            []string{"servo"},
		SetUpTimeout:    5 * time.Minute,
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		ServiceDeps:     []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Data:            []string{firmware.ConfigFile},
	})
	testing.AddFixture(&testing.Fixture{
		Name:            RecMode,
		Desc:            "Reboot into recovery mode before test",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeRecovery, false),
		Vars:            []string{"servo"},
		SetUpTimeout:    5 * time.Minute,
		ResetTimeout:    10 * time.Second,
		PreTestTimeout:  5 * time.Minute,
		TearDownTimeout: 5 * time.Minute,
		ServiceDeps:     []string{"tast.cros.firmware.BiosService", "tast.cros.firmware.UtilsService"},
		Data:            []string{firmware.ConfigFile},
	})
}

// Value contains fields that are useful for tests.
type Value struct {
	BootMode common.BootMode
	GBBFlags pb.GBBFlagsState
	Helper   *firmware.Helper
}

// impl contains fields that are useful for Fixture methods.
type impl struct {
	v            *Value
	origBootMode *common.BootMode
	origGBBFlags *pb.GBBFlagsState
}

// newFixture creates an instance of firmware Fixture.
func newFixture(mode common.BootMode, forceDev bool) testing.FixtureImpl {
	flags := pb.GBBFlagsState{Clear: common.AllGBBFlags(), Set: common.FAFTGBBFlags()}
	if forceDev {
		flags.Set = append(flags.Set, pb.GBBFlag_FORCE_DEV_SWITCH_ON, pb.GBBFlag_DEV_SCREEN_SHORT_DELAY)
	}
	return &impl{
		v: &Value{
			BootMode: mode,
			// Default GBBFlagsState for firmware testing.
			GBBFlags: flags,
		},
	}
}

// SetUp is called by the framework to set up the environment with possibly heavy-weight
// operations.
func (i *impl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	s.Log("Creating a new firmware Helper instance for fixture: ", i.String())
	i.initHelper(ctx, s)

	// If rebooting to recovery mode, verify the usb key.
	if i.v.BootMode == common.BootModeRecovery {
		if err := i.v.Helper.RequireServo(ctx); err != nil {
			s.Fatal("Could not connect to servod: ", err)
		}
		if err := i.v.Helper.SetupUSBKey(ctx, s.CloudStorage()); err != nil {
			s.Fatal("USBKey not working: ", err)
		}
	}
	return i.v
}

// Reset is called by the framework after each test (except for the last one) to do a
// light-weight reset of the environment to the original state.
func (i *impl) Reset(ctx context.Context) error {
	// Close the servo to reset pd role, watchdogs, etc.
	i.v.Helper.CloseServo(ctx)
	// Close the RPC client in case the DUT rebooted at some point, and it doesn't recover well.
	i.v.Helper.CloseRPCConnection(ctx)
	return nil
}

// PreTest is called by the framework before each test to do a light-weight set up for the test.
func (i *impl) PreTest(ctx context.Context, s *testing.FixtTestState) {
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

	// If this is the first PreTest invocation, save the starting boot mode.
	// This isn't in SetUp to avoid reading CurrentBootMode twice.
	if i.origBootMode == nil {
		s.Logf("Saving boot mode %q for restoration upon completion of all tests under this fixture", mode)
		i.origBootMode = &mode
	}

	if err := i.v.Helper.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Failed to require BiosServiceClient: ", err)
	}

	s.Log("Get current GBB flags")
	curr, err := i.v.Helper.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Getting current GBB Flags failed: ", err)
	}

	// If this is the first PreTest invocation, save the starting GBB flags.
	// This isn't in SetUp to avoid reading GetGBBFlags twice. (It's very slow)
	if i.origGBBFlags == nil {
		testing.ContextLogf(ctx, "Saving GBB flags %+v for restoration upon completion of all tests under this fixture", curr.Set)
		i.origGBBFlags = curr
	}

	rebootRequired := false
	if common.GBBFlagsStatesEqual(i.v.GBBFlags, *curr) {
		s.Log("GBBFlags are already proper")
	} else {
		s.Log("Setting GBB flags to ", i.v.GBBFlags.Set)
		if err := i.setAndCheckGBBFlags(ctx, i.v.GBBFlags); err != nil {
			s.Fatal("SetAndCheckGBBFlags failed: ", err)
		}
		if common.GBBFlagsChanged(*curr, i.v.GBBFlags, common.RebootRequiredGBBFlags()) {
			s.Log("Resetting DUT due to GBB flag change")
			rebootRequired = true
		}
	}

	if mode != i.v.BootMode {
		testing.ContextLogf(ctx, "Current boot mode is %q, rebooting to %q to satisfy fixture", mode, i.v.BootMode)
		rebootRequired = true
	}

	if rebootRequired {
		opts := []firmware.ModeSwitchOption{firmware.AssumeGBBFlagsCorrect}
		if i.v.ForcesDevMode() {
			opts = append(opts, firmware.AllowGBBForce)
		}
		if err := i.rebootToMode(ctx, i.v.BootMode, opts...); err != nil {
			s.Fatalf("Failed to reboot to mode %q: %s", i.v.BootMode, err)
		}
	}
}

// PostTest is called by the framework after each test to tear down changes PreTest made.
func (i *impl) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// TearDown is called by the framework to tear down the environment SetUp set up.
func (i *impl) TearDown(ctx context.Context, s *testing.FixtState) {
	defer func() {
		i.destroyHelper(ctx, s)
		i.origBootMode = nil
		i.origGBBFlags = nil
	}()

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

// String identifies this fixture.
func (i *impl) String() string {
	name := string(i.v.BootMode)
	if i.v.ForcesDevMode() {
		name += "-gbb"
	}
	return name
}

// initHelper ensures that the impl has a working Helper instance.
func (i *impl) initHelper(ctx context.Context, s *testing.FixtState) {
	if i.v.Helper == nil {
		servoSpec, _ := s.Var("servo")
		i.v.Helper = firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), servoSpec)
	}
}

// destroyHelper closes and nils any existing Helper instance.
func (i *impl) destroyHelper(ctx context.Context, s *testing.FixtState) {
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

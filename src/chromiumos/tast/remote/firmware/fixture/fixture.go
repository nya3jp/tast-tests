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
	NormalMode = "bootModeNormal"
	DevMode    = "bootModeDev"
	DevModeGBB = "bootModeDevGBB"
	RecMode    = "bootModeRec"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            NormalMode,
		Desc:            "Reboot into normal mode before test",
		Contacts:        []string{"tast-fw-library-reviewers@google.com", "jbettis@google.com"},
		Impl:            newFixture(common.BootModeNormal, false),
		Vars:            []string{"servo"},
		SetUpTimeout:    10 * time.Second,
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
		SetUpTimeout:    10 * time.Second,
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
		SetUpTimeout:    10 * time.Second,
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
		SetUpTimeout:    60 * time.Minute, // Setting up USB key is slow
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

// forcedDevMode reports whether the Precondition forces dev mode.
func (v *Value) forcedDevMode() bool {
	for _, flag := range v.GBBFlags.Set {
		if flag == pb.GBBFlag_FORCE_DEV_SWITCH_ON {
			return true
		}
	}
	return false
}

// impl contains fields that are useful for Fixture methods.
type impl struct {
	value        *Value
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
		value: &Value{
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
	if i.value.BootMode == common.BootModeRecovery {
		if err := i.value.Helper.RequireServo(ctx); err != nil {
			s.Fatal("Failed to connect to servod: ", err)
		}
		if err := i.value.Helper.SetupUSBKey(ctx, s.CloudStorage()); err != nil {
			s.Fatal("Failed to setup USB key: ", err)
		}
	}
	return i.value
}

// Reset is called by the framework after each test (except for the last one) to do a
// light-weight reset of the environment to the original state.
func (i *impl) Reset(ctx context.Context) error {
	// Close the servo to reset pd role, watchdogs, etc.
	i.value.Helper.CloseServo(ctx)
	// Close the RPC client in case the DUT rebooted at some point, and it doesn't recover well.
	i.value.Helper.CloseRPCConnection(ctx)
	return nil
}

// PreTest is called by the framework before each test to do a light-weight set up for the test.
func (i *impl) PreTest(ctx context.Context, s *testing.FixtTestState) {
	if err := i.value.Helper.RequireServo(ctx); err != nil {
		s.Fatal("Failed to connect to servod: ", err)
	}
	if err := i.value.Helper.EnsureDUTBooted(ctx); err != nil {
		s.Fatal("DUT is offline before test start: ", err)
	}

	// The GBB flags might prevent booting into the correct mode, so check the boot mode,
	// then save the GBB flags, then set the GBB flags, and finally reboot into the right mode.
	mode, err := i.value.Helper.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Fatal("Failed to get current boot mode: ", err)
	}

	// If this is the first PreTest invocation, save the starting boot mode.
	// This isn't in SetUp to avoid reading CurrentBootMode twice.
	if i.origBootMode == nil {
		s.Logf("Saving boot mode %q for restoration upon completion of all tests under this fixture", mode)
		i.origBootMode = &mode
	}

	if err := i.value.Helper.RequireBiosServiceClient(ctx); err != nil {
		s.Fatal("Failed to require BiosServiceClient: ", err)
	}

	s.Log("Get current GBB flags")
	curr, err := i.value.Helper.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to read GBB flags: ", err)
	}

	// If this is the first PreTest invocation, save the starting GBB flags.
	// This isn't in SetUp to avoid reading GetGBBFlags twice. (It's very slow)
	if i.origGBBFlags == nil {
		testing.ContextLogf(ctx, "Saving GBB flags %+v for restoration upon completion of all tests under this fixture", curr.Set)
		i.origGBBFlags = curr
	}

	rebootRequired := false
	if common.GBBFlagsStatesEqual(i.value.GBBFlags, *curr) {
		s.Log("GBBFlags are already proper")
	} else {
		s.Log("Setting GBB flags to ", i.value.GBBFlags.Set)
		if err := i.setAndCheckGBBFlags(ctx, i.value.GBBFlags); err != nil {
			s.Fatal("SetAndCheckGBBFlags failed: ", err)
		}
		if common.GBBFlagsChanged(*curr, i.value.GBBFlags, common.RebootRequiredGBBFlags()) {
			s.Log("Resetting DUT due to GBB flag change")
			rebootRequired = true
		}
	}

	if mode != i.value.BootMode {
		testing.ContextLogf(ctx, "Current boot mode is %q, rebooting to %q to satisfy fixture", mode, i.value.BootMode)
		rebootRequired = true
	}

	if rebootRequired {
		opts := []firmware.ModeSwitchOption{firmware.AssumeGBBFlagsCorrect}
		if i.value.forcedDevMode() {
			opts = append(opts, firmware.AllowGBBForce)
		}
		if err := i.rebootToMode(ctx, i.value.BootMode, opts...); err != nil {
			s.Fatalf("Failed to reboot to mode %q: %s", i.value.BootMode, err)
		}
	}
}

// PostTest is called by the framework after each test to tear down changes PreTest made.
func (i *impl) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// TearDown is called by the framework to tear down the environment SetUp set up.
func (i *impl) TearDown(ctx context.Context, s *testing.FixtState) {
	defer func(ctx context.Context) {
		i.closeHelper(ctx, s)
		i.origBootMode = nil
		i.origGBBFlags = nil
	}(ctx)

	// Close the servo to reset pd role, watchdogs, etc.
	i.value.Helper.CloseServo(ctx)
	// Close the RPC client in case the DUT rebooted at some point, and it doesn't recover well.
	i.value.Helper.CloseRPCConnection(ctx)

	if err := i.value.Helper.EnsureDUTBooted(ctx); err != nil {
		s.Fatal("DUT is offline after test end: ", err)
	}

	rebootRequired := false
	toMode := common.BootModeUnspecified
	if i.origGBBFlags != nil {
		if err := i.value.Helper.RequireBiosServiceClient(ctx); err != nil {
			s.Fatal("Failed to require BiosServiceClient: ", err)
		}

		testing.ContextLog(ctx, "Get current GBB flags")
		curr, err := i.value.Helper.BiosServiceClient.GetGBBFlags(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Getting current GBB Flags failed: ", err)
		}

		if !common.GBBFlagsStatesEqual(*i.origGBBFlags, *curr) {
			if err := i.setAndCheckGBBFlags(ctx, *i.origGBBFlags); err != nil {
				s.Fatal("Restore GBB flags failed: ", err)
			}
			if common.GBBFlagsChanged(*curr, i.value.GBBFlags, common.RebootRequiredGBBFlags()) {
				s.Log("Resetting DUT due to GBB flag change")
				rebootRequired = true
			}
		}
	}

	if i.origBootMode != nil {
		mode, err := i.value.Helper.Reporter.CurrentBootMode(ctx)
		if err != nil {
			s.Fatal("Failed to get boot mode: ", err)
		}
		if mode != *i.origBootMode {
			s.Logf("Restoring boot mode from %q to %q", mode, *i.origBootMode)
			rebootRequired = true
			toMode = *i.origBootMode
		}
	}
	if rebootRequired {
		opts := []firmware.ModeSwitchOption{firmware.AssumeGBBFlagsCorrect}
		if i.origGBBFlags != nil {
			for _, flag := range i.origGBBFlags.Set {
				if flag == pb.GBBFlag_FORCE_DEV_SWITCH_ON {
					opts = append(opts, firmware.AllowGBBForce)
					break
				}
			}
		}
		if err := i.rebootToMode(ctx, toMode, opts...); err != nil {
			s.Fatalf("Failed to reboot to mode %q: %s", toMode, err)
		}
	}
}

// String identifies this fixture.
func (i *impl) String() string {
	name := string(i.value.BootMode)
	if i.value.forcedDevMode() {
		name += "-gbb"
	}
	return name
}

// initHelper ensures that the impl has a working Helper instance.
func (i *impl) initHelper(ctx context.Context, s *testing.FixtState) {
	if i.value.Helper == nil {
		servoSpec, _ := s.Var("servo")
		i.value.Helper = firmware.NewHelper(s.DUT(), s.RPCHint(), s.DataPath(firmware.ConfigFile), servoSpec)
	}
}

// closeHelper closes and nils any existing Helper instance.
func (i *impl) closeHelper(ctx context.Context, s *testing.FixtState) {
	if i.value.Helper == nil {
		return
	}
	if err := i.value.Helper.Close(ctx); err != nil {
		s.Log("Failed to close helper: ", err)
	}
	i.value.Helper = nil
}

// rebootToMode reboots to the specified mode using the ModeSwitcher, it assumes the helper is present.
func (i *impl) rebootToMode(ctx context.Context, mode common.BootMode, opts ...firmware.ModeSwitchOption) error {
	ms, err := firmware.NewModeSwitcher(ctx, i.value.Helper)
	if err != nil {
		return errors.Wrap(err, "failed to create mode switcher")
	}
	if mode == common.BootModeUnspecified {
		if err := ms.ModeAwareReboot(ctx, firmware.WarmReset); err != nil {
			return errors.Wrap(err, "failed to warm reboot")
		}
	}
	if err := ms.RebootToMode(ctx, mode, opts...); err != nil {
		return errors.Wrapf(err, "failed to reboot to mode %q", mode)
	}
	return nil
}

// setAndCheckGBBFlags sets and reads back the GBBFlags to ensure correctness.
func (i *impl) setAndCheckGBBFlags(ctx context.Context, req pb.GBBFlagsState) error {
	if err := i.value.Helper.RequireBiosServiceClient(ctx); err != nil {
		return errors.Wrap(err, "failed to require bios service client")
	}

	if _, err := i.value.Helper.BiosServiceClient.ClearAndSetGBBFlags(ctx, &req); err != nil {
		return errors.Wrap(err, "failed to update GBB flags")
	}

	checker := checkers.New(i.value.Helper)
	if err := checker.GBBFlags(ctx, req); err != nil {
		return errors.Wrap(err, "gbb checker")
	}

	return nil
}

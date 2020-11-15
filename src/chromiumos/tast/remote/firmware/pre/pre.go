// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package pre implements preconditions for firmware tests.
package pre

import (
	"context"
	"time"

	common "chromiumos/tast/common/firmware"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/testing"
)

// Value contains fields that are useful for tests.
type Value struct {
	BootMode common.BootMode
	Helper   *firmware.Helper
}

// impl contains fields that are useful for Precondition methods.
type impl struct {
	v            *Value
	origBootMode *common.BootMode
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

const (
	// prepareTimeout is the maximum time that the Prepare method should take, adjust as needed.
	prepareTimeout = 5 * time.Minute
)

// newPrecondition creates an instance of firmware Precondition.
func newPrecondition(mode common.BootMode) testing.Precondition {
	return &impl{
		v: &Value{
			BootMode: mode,
		},
		timeout: prepareTimeout,
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

	// Can't Restore the boot mode if unknown.
	if i.origBootMode == nil {
		return
	}

	mode, err := i.v.Helper.Reporter.CurrentBootMode(ctx)
	if err != nil {
		s.Fatal("Could not get boot mode: ", err)
	}

	if mode != *i.origBootMode {
		s.Logf("Restoring boot mode from %q to %q", mode, *i.origBootMode)
		if err := i.rebootToMode(ctx, *i.origBootMode); err != nil {
			s.Fatal("Failed to restore boot mode: ", err)
		}
	}
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

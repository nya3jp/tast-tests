// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vpn

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shill"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "vpnShillReset",
		Desc: "A fixture that resets shill to a default state when this fixture starts and ends, and after a test if it failed",
		Contacts: []string{
			"jiejiang@google.com",        // fixture maintainer
			"cros-networking@google.com", // platform networking team
		},
		SetUpTimeout:    shill.ResetShillTimeout + 5*time.Second,
		PostTestTimeout: shill.ResetShillTimeout + 5*time.Second,
		TearDownTimeout: shill.ResetShillTimeout + 5*time.Second,
		Impl:            &vpnFixture{},
	})
}

func resetShillWithLockingHook(ctx context.Context) error {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us. Lock the hook
	// before shill restarted.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to lock check network hook")
	}
	defer unlock()

	if errs := shill.ResetShill(ctx); len(errs) != 0 {
		for _, err := range errs {
			testing.ContextLog(ctx, "ResetShill error: ", err)
		}
		return errors.Wrap(errs[0], "failed to reset shill")
	}

	return nil
}

type vpnFixture struct{}

func (f *vpnFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	if err := resetShillWithLockingHook(ctx); err != nil {
		s.Fatal("Failed to reset shill: ", err)
	}

	// Provides pass-through for the value yielded by the parent fixture.
	return s.ParentValue()
}

func (f *vpnFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *vpnFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *vpnFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if !s.HasError() {
		return
	}

	// Resets shill when the test failed. We assume that a successful test run
	// will not leave shill in a state which can affect the following tests.
	testing.ContextLog(ctx, "Test failed, reseting shill")
	if err := resetShillWithLockingHook(ctx); err != nil {
		s.Error("Failed to reset shill in PostTest: ", err)
	}
}

func (f *vpnFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Restart ui so that cryptohome unmounts all user mounts before shill is
	// restarted so that shill does not keep the mounts open perpetually.
	// TODO(b/205726835): Remove once the mount propagation for shill is fixed.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Error("Failed to restart ui: ", err)
	}

	if err := resetShillWithLockingHook(ctx); err != nil {
		s.Error("Failed to reset shill in TearDown: ", err)
	}
}

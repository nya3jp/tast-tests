// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package shill contains library code for interacting with shill that is
// specific to the network testing package.
package shill

import (
	"context"
	"os"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

// ResetShillTimeout specifies the timeout value for a shill restart.
const ResetShillTimeout = 30 * time.Second

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "shillReset",
		Desc: "A fixture that ensures shill is in a default state with no user profiles when the test starts and will reset any shill modifications after the test",
		Contacts: []string{
			"khegde@chromium.org",                 // fixture maintainer
			"stevenjb@chromium.org",               // fixture maintainer
			"cros-network-health-team@google.com", // Network Health team
		},
		PreTestTimeout:  ResetShillTimeout + 5*time.Second,
		PostTestTimeout: 5 * time.Second,
		TearDownTimeout: ResetShillTimeout + 5*time.Second,
		Impl:            &shillFixture{},
	})
	testing.AddFixture(&testing.Fixture{
		Name: "shillResetWithArcBooted",
		Desc: "A fixture that ensures shill is in a default state with no user profiles when the test starts and will reset any shill modifications after the test (with 'arcBooted' fixture)",
		Contacts: []string{
			"cassiewang@chromium.org",         // fixture maintainer
			"cros-networking-bugs@google.com", // platform networking team
		},
		PreTestTimeout:  ResetShillTimeout + 5*time.Second,
		PostTestTimeout: 5 * time.Second,
		TearDownTimeout: ResetShillTimeout + 5*time.Second,
		Impl:            &shillFixture{},
		Parent:          "arcBooted",
	})
}

// ResetShill does a best effort removing any modifications to the shill
// configuration and resetting it in a known default state.
func ResetShill(ctx context.Context) []error {
	var errs []error
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		errs = append(errs, errors.Wrap(err, "failed to stop shill"))
	}
	if err := os.Remove(shillconst.DefaultProfilePath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, errors.Wrap(err, "failed to remove default profile"))
	}
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		// No more can be done if shill doesn't start
		return append(errs, errors.Wrap(err, "failed to restart shill"))
	}
	return errs
}

// shillFixture implements testing.FixtureImpl.
type shillFixture struct {
	netUnlock func()
}

func (f *shillFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// Provides pass-through for the value yielded by the parent fixture.
	return s.ParentValue()
}

func (f *shillFixture) Reset(ctx context.Context) error {
	return nil
}

func (f *shillFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us. This is
	// automatically unlocked after 30 minutes, so unlock and lock it between
	// each test.
	success := false
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed to lock the check network hook: ", err)
	}
	defer func() {
		if !success {
			unlock()
		}
	}()

	if errs := ResetShill(ctx); len(errs) != 0 {
		for _, err := range errs {
			s.Error("ResetShill error: ", err)
		}
		s.Fatal("Failed resetting shill in PreTest")
	}

	// Ensure that no shill user profiles are loaded.
	m, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed to create Shill Manager: ", err)
	}
	if err = m.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to call Manager.PopAllUserProfiles: ", err)
	}

	// Ensure that a service is connected. Every DUT requires a primary connection,
	// so we uses this to ensure that normal Shill startup has completed.
	expectProps := map[string]interface{}{
		shillconst.ServicePropertyIsConnected: true,
	}
	if _, err := m.WaitForServiceProperties(ctx, expectProps, ResetShillTimeout); err != nil {
		s.Fatal("Failed to wait for connected service: ", err)
	}

	success = true
	f.netUnlock = unlock
}

func (f *shillFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	f.netUnlock()
}

func (f *shillFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	// Restart ui so that cryptohome unmounts all user mounts before shill is
	// restarted so that shill does not keep the mounts open perpetually.
	// TODO(b/205726835): Remove once the mount propagation for shill is fixed.
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Error("Failed to restart ui: ", err)
	}

	if errs := ResetShill(ctx); len(errs) != 0 {
		for _, err := range errs {
			s.Error("ResetShill error: ", err)
		}
		s.Error("Failed resetting shill in TearDown")
	}
}

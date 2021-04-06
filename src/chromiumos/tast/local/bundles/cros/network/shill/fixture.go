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
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name: "shillReset",
		Desc: "A fixture that ensures shill is in a default state when the test starts and will reset any shill modifications after the test",
		Contacts: []string{
			"tbegin@chromium.org",            // fixture author
			"stevenjb@chromium.org",          // network-health tech lead
			"cros-network-health@google.com", // network-health team
		},
		PreTestTimeout:  10 * time.Second,
		PostTestTimeout: 5 * time.Second,
		TearDownTimeout: 10 * time.Second,
		Impl:            &shillFixture{},
	})
}

// resetShill does a best effort removing any modifications to the shill
// configuration and resetting it in a known default state.
func resetShill(ctx context.Context) []error {
	var errs []error
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		errs = append(errs, err)
	}
	if err := os.Remove(shillconst.DefaultProfilePath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		// No more can be done if shill doesn't start
		return append(errs, err)
	}
	manager, err := shill.NewManager(ctx)
	if err != nil {
		// No more can be done if a manger interface cannot be created
		return append(errs, err)
	}
	if err = manager.PopAllUserProfiles(ctx); err != nil {
		errs = append(errs, err)
	}
	return errs
}

// shillFixture implements testing.FixtureImpl.
type shillFixture struct {
	netUnlock func()
}

func (f *shillFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	return nil
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

	if errs := resetShill(ctx); len(errs) != 0 {
		for _, err := range errs {
			s.Error("resetShill error: ", err)
		}
		s.Fatal("Failed resetting shill")
	}

	success = true
	f.netUnlock = unlock
}

func (f *shillFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	f.netUnlock()
}

func (f *shillFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	if errs := resetShill(ctx); len(errs) != 0 {
		for _, err := range errs {
			s.Error("resetShill error: ", err)
		}
		s.Error("Failed resetting shill")
	}
}

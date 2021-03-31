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
		SetUpTimeout:    20 * time.Second,
		PreTestTimeout:  10 * time.Second,
		ResetTimeout:    20 * time.Second,
		TearDownTimeout: 5 * time.Second,
		Impl:            &shillFixture{},
	})
}

// shillFixture implements testing.FixtureImpl.
type shillFixture struct {
	netUnlock func()
}

func (f *shillFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
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

	if err := f.Reset(ctx); err != nil {
		s.Fatal("Unable to reset shill state: ", err)
	}

	success = true
	f.netUnlock = unlock
	return nil
}

func (f *shillFixture) Reset(ctx context.Context) error {
	if err := upstart.StopJob(ctx, "shill"); err != nil {
		return err
	}
	if err := os.Remove(shillconst.DefaultProfilePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := upstart.RestartJob(ctx, "shill"); err != nil {
		return err
	}
	return nil
}

func (f *shillFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	if err = manager.PopAllUserProfiles(ctx); err != nil {
		s.Fatal("Failed to pop user profiles: ", err)
	}
}

func (f *shillFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *shillFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	f.netUnlock()
}

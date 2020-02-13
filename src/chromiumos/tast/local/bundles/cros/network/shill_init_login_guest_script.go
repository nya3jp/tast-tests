// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shillscript"
	"chromiumos/tast/local/network"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillInitLoginGuestScript,
		Desc:     "Test that shill init login guest script perform as expected",
		Contacts: []string{"arowa@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ShillInitLoginGuestScript(ctx context.Context, s *testing.State) {
	// We lose connectivity along the way here, and if that races with the
	// recover_duts network-recovery hooks, it may interrupt us.
	unlock, err := network.LockCheckNetworkHook(ctx)
	if err != nil {
		s.Fatal("Failed locking the check network hook: ", err)
	}
	defer unlock()

	var env shillscript.TestEnv

	defer shillscript.TearDown(ctx, &env)

	if err := shillscript.SetUp(ctx, &env); err != nil {
		s.Fatal("Failed starting the test: ", err)
	}

	if err := testLoginGuest(ctx, &env); err != nil {
		s.Fatal("Failed running testLoginGuest")
	}
}

// testLoginGuest tests the guest login process.
// Login should create a temporary profile directory in /run,
// instead of using one within the root directory for normal users.
func testLoginGuest(ctx context.Context, env *shillscript.TestEnv) error {
	// Simulate guest login.
	// For guest login, session-manager passes an empty CHROMEOS_USER arg.
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, shillscript.DbusMonitorTimeout)
	defer cancel()

	stop, err := shillscript.DbusEventMonitor(timeoutCtx)
	if err != nil {
		return err
	}

	if err := shillscript.Login(ctx, ""); err != nil {
		return errors.Wrap(err, "failed logging in")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{shillscript.CreateUserProfile, shillscript.InsertUserProfile}
	if err := shillscript.AssureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	if err := shillscript.AssureNotExists(env.ShillUserProfile); err != nil {
		return errors.Wrapf(err, "failed shill user profile %s does exist", env.ShillUserProfile)
	}

	if err := shillscript.AssureNotExists(env.ShillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed shill user profile directory %s does exist", env.ShillUserProfileDir)
	}

	if err := shillscript.AssureIsDir(shillscript.GuestShillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", shillscript.GuestShillUserProfileDir)
	}

	if err := shillscript.AssureIsDir("/run/shill/user_profiles"); err != nil {
		return errors.Wrap(err, "failed asserting that /run/shill/user_profiles is a directory")
	}

	if err := shillscript.AssureIsLinkTo("/run/shill/user_profiles/chronos", shillscript.GuestShillUserProfileDir); err != nil {
		return err
	}

	if err := shillscript.AssureIsDir(shillscript.GuestShillUserLogDir); err != nil {
		return errors.Wrapf(err, "failed asserting that %v is a directory", shillscript.GuestShillUserLogDir)
	}

	if err := shillscript.AssureIsLinkTo("/run/shill/log", shillscript.GuestShillUserLogDir); err != nil {
		return err
	}

	profiles, err := shillscript.GetProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		return errors.Wrap(err, "profile list is empty")
	}

	// The last profile should be the one we just created.
	profilePath := profiles[len(profiles)-1].String()

	if profilePath != shillscript.ExpectedProfileName {
		return errors.Wrapf(err, "found unexpected profile path: got %s, want %s", profilePath, shillscript.ExpectedProfileName)
	}

	return nil
}

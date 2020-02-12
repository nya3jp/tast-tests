// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shillscript"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillInitScriptsLoginProfileExists,
		Desc:     "Test that shill init scripts perform as expected",
		Contacts: []string{"arowa@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ShillInitScriptsLoginProfileExists(ctx context.Context, s *testing.State) {
	if err := shillscript.RunTest(ctx, testLoginProfileExists); err != nil {
		s.Fatal("Failed running testLoginProfileExists: ", err)
	}
}

// testLoginProfileExists tests logging in a user whose profile already exists.
// Login script should only push (and not create) the user profile
// if a user profile already exists.
func testLoginProfileExists(ctx context.Context, env *shillscript.TestEnv) error {
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	if err := os.Mkdir(env.ShillUserProfileDir, 0700); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", env.ShillUserProfileDir)
	}

	if err := testexec.CommandContext(ctx, "chown", "shill:shill", env.ShillUserProfileDir).Run(); err != nil {
		return errors.Wrapf(err, "failed changing the owner of the directory %s to shill", shillscript.ShillUserProfilesDir)
	}

	if err := os.Mkdir(shillscript.ShillUserProfilesDir, 0700); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", shillscript.ShillUserProfilesDir)
	}

	if err := testexec.CommandContext(ctx, "chown", "shill:shill", shillscript.ShillUserProfilesDir).Run(); err != nil {
		return errors.Wrapf(err, "failed changing the owner of the directory %s to shill", shillscript.ShillUserProfilesDir)
	}

	if err := os.Symlink(env.ShillUserProfileDir, shillscript.ShillUserProfileChronosDir); err != nil {
		return errors.Wrapf(err, "failed to symlink %s to %s", env.ShillUserProfileDir, shillscript.ShillUserProfileChronosDir)
	}

	if err := shillscript.CreateProfile(ctx, shillscript.ChronosProfileName); err != nil {
		return err
	}

	// CreateProfile will create the profile link directory (shillscript.ShillUserProfileChronosDir).
	// The login script will exit early and not run, if the profile link directory exists, because it
	// shoudn't load multiple network profiles. For that reason, the profile link directory is removed
	// before running the login script.
	if err := os.RemoveAll(shillscript.ShillUserProfileChronosDir); err != nil {
		return errors.Wrapf(err, "failed removing %s", shillscript.ShillUserProfileChronosDir)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, shillscript.DbusMonitorTimeout)
	defer cancel()

	stop, err := shillscript.DbusEventMonitor(timeoutCtx)
	if err != nil {
		return err
	}

	if err := shillscript.Login(ctx, shillscript.FakeUser); err != nil {
		_, _ = stop()
		return errors.Wrap(err, "failed logging in")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{shillscript.InsertUserProfile}
	if err := shillscript.AssureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	profiles, err := shillscript.GetProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) != 2 {
		return errors.Wrapf(err, "found unexpected number of profiles in the profile stack: got %d, want 2", len(profiles))
	}

	return nil
}

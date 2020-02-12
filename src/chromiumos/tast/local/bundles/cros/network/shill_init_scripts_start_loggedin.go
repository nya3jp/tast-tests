// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"os"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shillscript"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ShillInitScriptsStartLoggedin,
		Desc:     "Test that shill init scripts perform as expected",
		Contacts: []string{"arowa@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ShillInitScriptsStartLoggedin(ctx context.Context, s *testing.State) {
	if err := shillscript.RunTest(ctx, testStartLoggedIn); err != nil {
		s.Fatal("Failed running testStartLoggedIn: ", err)
	}
}

// testStartLoggedIn tests starting up shill while user is already logged in.
func testStartLoggedIn(ctx context.Context, env *shillscript.TestEnv) error {
	if err := os.Mkdir("/run/shill", os.ModePerm); err != nil {
		return errors.Wrap(err, "failed making the directory /run/shill")
	}

	if err := os.Mkdir(shillscript.ShillUserProfilesDir, os.ModePerm); err != nil {
		return errors.Wrapf(err, "failed making the directory %s", shillscript.ShillUserProfilesDir)
	}

	if err := shillscript.CreateShillUserProfile("", env); err != nil {
		return errors.Wrap(err, "failed creating the shill user profile")
	}

	if err := os.Symlink(env.ShillUserProfileDir, "/run/shill/user_profiles/chronos"); err != nil {
		return errors.Wrapf(err, "failed to symlink %s to /run/shill/user_profiles/chronos", env.ShillUserProfileDir)
	}

	if err := shillscript.Touch("/run/state/logged-in"); err != nil {
		return err
	}

	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	if err := os.Remove("/run/state/logged-in"); err != nil {
		return errors.Wrap(err, "failed to remove /run/state/logged-in")
	}

	return nil
}

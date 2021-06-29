// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/shillscript"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShillInitScriptsLoginMultiProfile,
		Desc:         "Test that shill init login script perform as expected",
		Contacts:     []string{"arowa@google.com", "cros-networking@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
	})
}

func ShillInitScriptsLoginMultiProfile(ctx context.Context, s *testing.State) {
	if err := shillscript.RunTest(ctx, testLoginMultiProfile, false); err != nil {
		s.Fatal("Failed running testLoginMultiProfile: ", err)
	}
}

// testLoginMultiProfile tests signalling shill about multiple logged-in users.
// Login script should not create multiple profiles in parallel
// if called more than once without an intervening logout.  Only
// the initial user profile should be created.
func testLoginMultiProfile(ctx context.Context, env *shillscript.TestEnv) error {
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	// First logged-in user should create a profile (tested above).
	cr, err := chrome.New(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	cr.Close(ctx)

	for i := 0; i < 5; i++ {
		timeoutCtx, cancel := context.WithTimeout(ctx, shillscript.DbusMonitorTimeout)
		defer cancel()

		stop, err := shillscript.DbusEventMonitor(timeoutCtx)
		if err != nil {
			return err
		}

		if err := login(ctx, shillscript.FakeUser); err != nil {
			stop()
			return errors.Wrap(err, "Chrome failed to log in")
		}

		calledMethods, err := stop()
		if err != nil {
			return err
		}

		var expectedCalls []string
		if err := shillscript.AssureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
			return err
		}

		files, err := ioutil.ReadDir(shillscript.ShillUserProfilesDir)
		if err != nil {
			return err
		}
		if len(files) != 1 {
			return errors.Errorf("found unexpected number of profiles in the directory %s: got %v, want 1", shillscript.ShillUserProfilesDir, len(files))
		}
		if files[0].Name() != "chronos" {
			return errors.Errorf("found unexpected profile link in the directory %s: got %v, want chronos", shillscript.ShillUserProfilesDir, files[0].Name())
		}
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

// login simulates the login process.
func login(ctx context.Context, user string) error {
	if err := upstart.StartJob(ctx, "shill-start-user-session", upstart.WithArg("CHROMEOS_USER", user)); err != nil {
		return err
	}
	return nil
}

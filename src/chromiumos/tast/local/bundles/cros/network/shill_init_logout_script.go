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
		Func:     ShillInitLogoutScript,
		Desc:     "Test that shill init logout script perform as expected",
		Contacts: []string{"arowa@google.com", "cros-networking@google.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

func ShillInitLogoutScript(ctx context.Context, s *testing.State) {
	if err := shillscript.RunTest(ctx, testLogout); err != nil {
		s.Fatal("Failed running testLogout: ", err)
	}
}

// testLogout tests the logout process.
func testLogout(ctx context.Context, env *shillscript.TestEnv) error {
	if err := upstart.StartJob(ctx, "shill"); err != nil {
		return errors.Wrap(err, "failed starting shill")
	}

	if err := os.MkdirAll(shillscript.ShillUserProfilesDir, 0777); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", shillscript.ShillUserProfilesDir)
	}

	if err := os.MkdirAll(shillscript.GuestShillUserProfileDir, 0777); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", shillscript.GuestShillUserProfileDir)
	}

	if err := os.MkdirAll(shillscript.GuestShillUserLogDir, 0777); err != nil {
		return errors.Wrapf(err, "failed creating the directory: %s", shillscript.GuestShillUserLogDir)
	}

	if err := shillscript.Touch("/run/state/logged-in"); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, shillscript.DbusMonitorTimeout)
	defer cancel()

	stop, err := shillscript.DbusEventMonitor(timeoutCtx)
	if err != nil {
		return err
	}

	if err := shillscript.Logout(ctx); err != nil {
		_, _ = stop()
		return errors.Wrap(err, "failed logging out")
	}

	calledMethods, err := stop()
	if err != nil {
		return err
	}

	expectedCalls := []string{shillscript.PopAllUserProfiles}
	if err := shillscript.AssureMethodCalls(ctx, expectedCalls, calledMethods); err != nil {
		return err
	}

	if err := shillscript.AssureNotExists(shillscript.ShillUserProfilesDir); err != nil {
		return errors.Wrapf(err, "shill user profile %s exists", shillscript.ShillUserProfilesDir)
	}

	if err := shillscript.AssureNotExists(shillscript.GuestShillUserProfileDir); err != nil {
		return errors.Wrapf(err, "failed guest shill user profile directory %s exists", shillscript.GuestShillUserProfileDir)
	}

	if err := shillscript.AssureNotExists(shillscript.GuestShillUserLogDir); err != nil {
		return errors.Wrapf(err, "failed guest shill user log directory %s exists", shillscript.GuestShillUserLogDir)
	}

	profiles, err := shillscript.GetProfileList(ctx)
	if err != nil {
		return err
	}

	if len(profiles) > 1 {
		return errors.Wrapf(err, "found unexpected number of profiles in the profile stack: got %d, want 1", len(profiles))
	}

	return nil
}
